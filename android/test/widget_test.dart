// Smoke test for snowden_android.
//
// The default Flutter counter template referenced a MyApp class that does not
// exist in the real app. Replace it with a minimal check that the production
// root widget builds without throwing and renders the configured title.
//
// HomeScreen uses platform channels (VpnService / credential provider), so
// there is no point asserting on internal counter state. We only guarantee
// that the widget tree compiles and pumpWidget returns, which is enough to
// keep `flutter test`, `flutter analyze` and the gradle assemble pipeline
// happy.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:snowden_android/main.dart';

void main() {
  testWidgets('SnowdenApp boots and renders root MaterialApp',
      (WidgetTester tester) async {
    await tester.pumpWidget(const SnowdenApp());
    await tester.pump();

    final BuildContext context = tester.element(find.byType(MaterialApp));
    final MaterialApp app = context.widget as MaterialApp;

    expect(app.title, 'snowden.system');
    expect(app.debugShowCheckedModeBanner, isFalse);
  });
}
