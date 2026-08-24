import 'package:flutter_test/flutter_test.dart';
import 'package:gnas_app/main.dart';

void main() {
  testWidgets('App launches with login page', (WidgetTester tester) async {
    await tester.pumpWidget(const GnasApp());
    expect(find.text('GNAS'), findsOneWidget);
    expect(find.text('登 录'), findsOneWidget);
  });
}