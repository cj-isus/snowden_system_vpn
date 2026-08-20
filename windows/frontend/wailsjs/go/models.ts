export namespace core {
	
	export class DiagStatus {
	    state: string;
	    category: string;
	    explanation: string;
	    lastError: string;
	    failCount: number;
	    okCount: number;
	
	    static createFrom(source: any = {}) {
	        return new DiagStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.category = source["category"];
	        this.explanation = source["explanation"];
	        this.lastError = source["lastError"];
	        this.failCount = source["failCount"];
	        this.okCount = source["okCount"];
	    }
	}
	export class DomainScore {
	    domain: string;
	    bestOutbound: string;
	    score: number;
	    requests: number;
	    avgLatencyMs: number;
	    successRate: number;
	
	    static createFrom(source: any = {}) {
	        return new DomainScore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domain = source["domain"];
	        this.bestOutbound = source["bestOutbound"];
	        this.score = source["score"];
	        this.requests = source["requests"];
	        this.avgLatencyMs = source["avgLatencyMs"];
	        this.successRate = source["successRate"];
	    }
	}
	export class RouteRuleInfo {
	    id: string;
	    icon: string;
	    title: string;
	    sub: string;
	    route: string;
	    on: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RouteRuleInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.icon = source["icon"];
	        this.title = source["title"];
	        this.sub = source["sub"];
	        this.route = source["route"];
	        this.on = source["on"];
	    }
	}
	export class ServerInfo {
	    id: string;
	    name: string;
	    protocol: string;
	    server: string;
	    port: number;
	    location: string;
	    active: boolean;
	    ping: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.protocol = source["protocol"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.location = source["location"];
	        this.active = source["active"];
	        this.ping = source["ping"];
	    }
	}
	export class TrafficStats {
	    available: boolean;
	    downloadSpeed: number;
	    uploadSpeed: number;
	    downloadTotal: number;
	    uploadTotal: number;
	    uptime: number;
	
	    static createFrom(source: any = {}) {
	        return new TrafficStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.downloadSpeed = source["downloadSpeed"];
	        this.uploadSpeed = source["uploadSpeed"];
	        this.downloadTotal = source["downloadTotal"];
	        this.uploadTotal = source["uploadTotal"];
	        this.uptime = source["uptime"];
	    }
	}
	export class VPNStatus {
	    state: string;
	    configId: string;
	    message: string;
	    connected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VPNStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.configId = source["configId"];
	        this.message = source["message"];
	        this.connected = source["connected"];
	    }
	}

}

