import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message_href.dart';

void main() {
  group('isAllowedHref', () {
    test('https с пустым хостом — false', () {
      expect(isAllowedHref('https:///path'), false);
    });

    test('http с хостом — true', () {
      expect(isAllowedHref('http://example.com/x'), true);
    });

    test('mailto с адресом в path — true', () {
      expect(isAllowedHref('mailto:user@example.com'), true);
    });

    test('mailto без адреса — false', () {
      expect(isAllowedHref('mailto:'), false);
    });

    test('javascript — false', () {
      expect(isAllowedHref('javascript:void(0)'), false);
    });

    test('data: — false', () {
      expect(isAllowedHref('data:text/plain,xx'), false);
    });
  });
}
