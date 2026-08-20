package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ValidationOptions controls how strict Validate is. Published examples may
// allow placeholders, while a runtime config must reject them and enforce the
// protected selector contract.
type ValidationOptions struct {
	AllowPlaceholders bool
	RequireFailClosed bool
}

// ValidationError contains every discovered config problem instead of stopping
// at the first one. This makes diagnostics useful to both humans and tooling.
type ValidationError struct {
	Issues []string
}

func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "config validation failed"
	}
	return "config validation failed: " + strings.Join(e.Issues, "; ")
}

// Validate checks structural references and the protected-route safety policy.
// It deliberately validates only the generic graph around sing-box options;
// protocol-specific schema validation remains the responsibility of the exact
// embedded sing-box build in box.New.
func Validate(data []byte, opts ValidationOptions) error {
	root, err := decodeObject(data)
	if err != nil {
		return err
	}

	var issues []string
	if !opts.AllowPlaceholders {
		collectPlaceholders(root, "$", &issues)
	}

	outbounds := objectList(root["outbounds"])
	endpoints := objectList(root["endpoints"])
	inbounds := objectList(root["inbounds"])

	outboundTags, _ := collectTags(outbounds, "outbounds", &issues)
	endpointTags, _ := collectTags(endpoints, "endpoints", &issues)
	inboundTags, _ := collectTags(inbounds, "inbounds", &issues)

	allRouteTags := make(map[string]bool, len(outboundTags)+len(endpointTags))
	for tag := range outboundTags {
		allRouteTags[tag] = true
	}
	for tag := range endpointTags {
		allRouteTags[tag] = true
	}

	for i, outbound := range outbounds {
		typ, _ := outbound["type"].(string)
		for _, child := range stringList(outbound["outbounds"]) {
			if !outboundTags[child] {
				issues = append(issues, fmt.Sprintf("outbounds[%d].outbounds references missing tag %q", i, child))
			}
		}
		if (typ == "selector" || typ == "urltest") && len(stringList(outbound["outbounds"])) == 0 {
			issues = append(issues, fmt.Sprintf("outbounds[%d] (%s) has no candidate outbounds", i, typ))
		}
	}

	route, _ := root["route"].(map[string]any)
	if route == nil {
		issues = append(issues, "route object is missing")
	} else {
		final, _ := route["final"].(string)
		if final == "" {
			issues = append(issues, "route.final is missing")
		} else if !allRouteTags[final] {
			issues = append(issues, fmt.Sprintf("route.final references missing tag %q", final))
		}
		if opts.RequireFailClosed && final == "direct" {
			issues = append(issues, "route.final cannot be direct in protected runtime mode")
		}

		for i, rule := range objectList(route["rules"]) {
			if outbound, ok := rule["outbound"].(string); ok && outbound != "any" && !allRouteTags[outbound] {
				issues = append(issues, fmt.Sprintf("route.rules[%d].outbound references missing tag %q", i, outbound))
			}
			if inbound, ok := rule["inbound"].(string); ok && inbound != "any" && !inboundTags[inbound] {
				issues = append(issues, fmt.Sprintf("route.rules[%d].inbound references missing tag %q", i, inbound))
			}
		}
	}

	dns, _ := root["dns"].(map[string]any)
	if dns != nil {
		dnsTags := make(map[string]bool)
		for i, server := range objectList(dns["servers"]) {
			tag, _ := server["tag"].(string)
			if tag == "" {
				issues = append(issues, fmt.Sprintf("dns.servers[%d].tag is missing", i))
				continue
			}
			if dnsTags[tag] {
				issues = append(issues, fmt.Sprintf("duplicate dns server tag %q", tag))
			}
			dnsTags[tag] = true
			if detour, ok := server["detour"].(string); ok && detour != "" && detour != "direct" && detour != "any" && !allRouteTags[detour] {
				issues = append(issues, fmt.Sprintf("dns.servers[%d].detour references missing tag %q", i, detour))
			}
		}
		for i, rule := range objectList(dns["rules"]) {
			if server, ok := rule["server"].(string); ok && server != "" && server != "local" && server != "fakeip" && !dnsTags[server] {
				issues = append(issues, fmt.Sprintf("dns.rules[%d].server references missing DNS tag %q", i, server))
			}
		}
	}

	if opts.RequireFailClosed {
		selectorFound := false
		for i, outbound := range outbounds {
			if outbound["type"] != "selector" || outbound["tag"] != "proxy" {
				continue
			}
			selectorFound = true
			candidates := stringList(outbound["outbounds"])
			if len(candidates) == 0 {
				issues = append(issues, "protected selector proxy has no candidates")
			}
			for _, candidate := range candidates {
				if candidate == "direct" {
					issues = append(issues, fmt.Sprintf("outbounds[%d].outbounds cannot contain direct", i))
				}
			}
			def, _ := outbound["default"].(string)
			if def == "" {
				issues = append(issues, "protected selector proxy must define a default candidate")
			} else if !contains(candidates, def) {
				issues = append(issues, fmt.Sprintf("protected selector default %q is not a candidate", def))
			} else if def == "direct" {
				issues = append(issues, "protected selector default cannot be direct")
			}
		}
		// Endpoint-only configurations (for example a standalone validated WARP
		// endpoint) may not need a selector. If a selector exists, however, it is
		// the mandatory owner of protected fallback policy.
		_ = selectorFound
	}

	if len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	return nil
}

// SetProtectedSelectorDefault changes only the selected protected outbound.
// It is used by the controller after the config has already been normalized.
func SetProtectedSelectorDefault(data []byte, channelTag string) ([]byte, error) {
	root, err := decodeObject(data)
	if err != nil {
		return nil, err
	}
	for _, outbound := range objectList(root["outbounds"]) {
		if outbound["type"] != "selector" || outbound["tag"] != "proxy" {
			continue
		}
		candidates := stringList(outbound["outbounds"])
		if !contains(candidates, channelTag) || channelTag == "direct" || channelTag == "block" {
			return nil, fmt.Errorf("protected channel %q is not a selector candidate", channelTag)
		}
		outbound["default"] = channelTag
		return json.MarshalIndent(root, "", "  ")
	}
	return nil, fmt.Errorf("protected selector proxy is missing")
}

// NormalizeProtectedRoute converts a legacy urltest-owned protected route into
// the target architecture: selector "proxy" owns the validated candidates and
// direct is retained only as an explicit route action. The diagnostic urltest
// is kept but removed from the live route and from direct fallback candidates.
func NormalizeProtectedRoute(data []byte) ([]byte, error) {
	root, err := decodeObject(data)
	if err != nil {
		return nil, err
	}

	outbounds := objectList(root["outbounds"])
	byTag := make(map[string]map[string]any, len(outbounds))
	for _, outbound := range outbounds {
		if tag, ok := outbound["tag"].(string); ok && tag != "" {
			byTag[tag] = outbound
		}
	}

	urltestTag := ""
	var candidates []string
	for _, outbound := range outbounds {
		if outbound["type"] != "urltest" {
			continue
		}
		if urltestTag == "" {
			urltestTag, _ = outbound["tag"].(string)
		}
		if tag, _ := outbound["tag"].(string); tag == "auto" {
			urltestTag = tag
			candidates = stringList(outbound["outbounds"])
			break
		}
	}
	if len(candidates) == 0 && urltestTag != "" {
		candidates = stringList(byTag[urltestTag]["outbounds"])
	}

	filtered := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		ob := byTag[candidate]
		if ob == nil {
			return nil, fmt.Errorf("urltest candidate %q is missing from outbounds", candidate)
		}
		typ, _ := ob["type"].(string)
		if typ == "direct" || typ == "block" || typ == "selector" || typ == "urltest" || typ == "dns" {
			continue
		}
		filtered = append(filtered, candidate)
	}

	if len(filtered) == 0 {
		// A config with no urltest can already be selector/endpoint based. In that
		// case leave it untouched; otherwise failing early is safer than inventing
		// a direct fallback.
		if len(candidates) > 0 {
			return nil, errorsf("protected urltest has no non-direct candidates")
		}
		return data, nil
	}

	selector := byTag["proxy"]
	if selector == nil {
		selector = map[string]any{
			"type":    "selector",
			"tag":     "proxy",
			"default": filtered[0],
		}
		outbounds = append(outbounds, selector)
		root["outbounds"] = outbounds
	}
	selector["type"] = "selector"
	selector["outbounds"] = stringSliceAny(filtered)
	if def, ok := selector["default"].(string); !ok || !contains(filtered, def) {
		selector["default"] = filtered[0]
	}

	// Keep urltest available only as a diagnostic group, never as a live route.
	if urltest := byTag[urltestTag]; urltest != nil {
		urltest["outbounds"] = stringSliceAny(filtered)
	}

	route, _ := root["route"].(map[string]any)
	if route == nil {
		route = map[string]any{}
		root["route"] = route
	}
	if final, _ := route["final"].(string); final == urltestTag || final == "auto" || final == "" {
		route["final"] = "proxy"
	}
	for _, rule := range objectList(route["rules"]) {
		if outbound, ok := rule["outbound"].(string); ok && (outbound == urltestTag || outbound == "auto") {
			rule["outbound"] = "proxy"
		}
	}

	if dns, ok := root["dns"].(map[string]any); ok {
		for _, server := range objectList(dns["servers"]) {
			if detour, ok := server["detour"].(string); ok && (detour == urltestTag || detour == "auto") {
				server["detour"] = "proxy"
			}
		}
	}

	return json.MarshalIndent(root, "", "  ")
}

func decodeObject(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("decode config JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode config JSON: trailing data")
		}
		return nil, fmt.Errorf("decode config JSON: trailing data: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("decode config JSON: root must be an object")
	}
	return root, nil
}

func objectList(value any) []map[string]any {
	items, _ := value.([]any)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			result = append(result, object)
		}
	}
	return result
}

func stringList(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := item.(string); ok {
			result = append(result, value)
		}
	}
	return result
}

func stringSliceAny(values []string) []any {
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func collectTags(items []map[string]any, section string, issues *[]string) (map[string]bool, map[string]string) {
	tags := make(map[string]bool, len(items))
	types := make(map[string]string, len(items))
	for i, item := range items {
		tag, _ := item["tag"].(string)
		if tag == "" {
			*issues = append(*issues, fmt.Sprintf("%s[%d].tag is missing", section, i))
			continue
		}
		if tags[tag] {
			*issues = append(*issues, fmt.Sprintf("duplicate %s tag %q", section, tag))
		}
		tags[tag] = true
		typ, _ := item["type"].(string)
		types[tag] = typ
	}
	return tags, types
}

func collectPlaceholders(value any, path string, issues *[]string) {
	switch typed := value.(type) {
	case string:
		upper := strings.ToUpper(typed)
		for _, marker := range []string{"YOUR_", "REPLACE_WITH", "CHANGE_ME", "EXAMPLE_ONLY"} {
			if strings.Contains(upper, marker) {
				*issues = append(*issues, fmt.Sprintf("%s contains placeholder %q", path, marker))
				return
			}
		}
	case map[string]any:
		for key, child := range typed {
			collectPlaceholders(child, path+"."+key, issues)
		}
	case []any:
		for i, child := range typed {
			collectPlaceholders(child, fmt.Sprintf("%s[%d]", path, i), issues)
		}
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func errorsf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
