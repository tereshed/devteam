import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/routing/auth_guard.dart';
import 'package:frontend/features/auth/domain/models/user_model.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:go_router/go_router.dart';
import 'package:mocktail/mocktail.dart';

class _LoadingForever extends AuthController {
  @override
  Future<UserModel?> build() async {
    await Completer<void>().future;
    return null;
  }
}

class MockGoRouterState extends Mock implements GoRouterState {}

void main() {
  late MockGoRouterState mockState;

  setUp(() {
    mockState = MockGoRouterState();
  });

  testWidgets(
    'authGuard: при AsyncLoading не редиректит (null) — контракт для rootRouterRedirect',
    (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(_LoadingForever.new),
          ],
          child: const MaterialApp(home: Scaffold(body: SizedBox())),
        ),
      );
      await tester.pump();
      final ctx = tester.element(find.byType(SizedBox));
      expect(authGuard(ctx, mockState), isNull);
    },
  );
}
