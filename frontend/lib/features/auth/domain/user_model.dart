import 'package:freezed_annotation/freezed_annotation.dart';

part 'user_model.freezed.dart';
part 'user_model.g.dart';

/// UserModel представляет модель пользователя
///
/// Используется для передачи данных пользователя между слоями приложения.
@freezed
abstract class UserModel with _$UserModel {
  const factory UserModel({
    required String id,
    required String email,
    required String role,
    @Default(false) @JsonKey(name: 'email_verified') bool emailVerified,
  }) = _UserModel;

  const UserModel._();

  /// Создать UserModel из JSON (ответ API)
  factory UserModel.fromJson(Map<String, dynamic> json) =>
      _$UserModelFromJson(json);
}
