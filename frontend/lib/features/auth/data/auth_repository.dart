import 'package:dio/dio.dart';
import 'package:frontend/features/auth/domain/auth_exceptions.dart';
import 'package:frontend/features/auth/domain/models.dart';

/// AuthRepository отвечает за работу с API авторизации
///
/// Абстрагирует логику получения данных от API от бизнес-логики.
/// Используется в контроллерах для получения данных.
class AuthRepository {
  final Dio _dio;

  AuthRepository({required Dio dio}) : _dio = dio;

  /// Регистрация нового пользователя
  Future<Map<String, dynamic>> register({
    required String email,
    required String password,
  }) async {
    try {
      final response = await _dio.post(
        '/auth/register',
        data: {'email': email, 'password': password},
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Вход пользователя
  Future<Map<String, dynamic>> login({
    required String email,
    required String password,
  }) async {
    try {
      final response = await _dio.post(
        '/auth/login',
        data: {'email': email, 'password': password},
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Обновление access token используя refresh token
  Future<Map<String, dynamic>> refreshToken({
    required String refreshToken,
  }) async {
    try {
      final response = await _dio.post(
        '/auth/refresh',
        data: {'refresh_token': refreshToken},
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Получение данных текущего пользователя
  Future<UserModel> getCurrentUser() async {
    try {
      final response = await _dio.get('/auth/me');
      return UserModel.fromJson(response.data as Map<String, dynamic>);
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Выход пользователя
  Future<void> logout() async {
    try {
      await _dio.post('/auth/logout');
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Обработка ошибок Dio
  AuthException _handleError(DioException error) {
    if (error.response != null) {
      final data = error.response!.data;
      String? errorCode;
      String? errorMessage;

      if (data is Map<String, dynamic>) {
        errorCode = data['error'] as String?;
        errorMessage = data['message'] as String?;
      }

      final message = errorMessage ?? error.message;

      // Сначала пытаемся определить ошибку по коду
      if (errorCode != null) {
        switch (errorCode) {
          case 'invalid_credentials':
            return InvalidCredentialsException(message);
          case 'user_not_found':
            return UserNotFoundException(message);
          case 'user_already_exists':
            return UserAlreadyExistsException(message);
          case 'access_denied':
            return AccessDeniedException(message);
          case 'invalid_token':
          case 'token_expired':
          case 'token_required':
          case 'invalid_auth_header':
            return const AccessDeniedException('Session expired or invalid');
        }
      }

      // Если кода нет или он неизвестен, используем статус код
      final statusCode = error.response!.statusCode;
      switch (statusCode) {
        case 400:
          // Often means validation error or bad request
          return UnknownAuthException(message);
        case 401:
          return InvalidCredentialsException(message);
        case 403:
          return AccessDeniedException(message);
        case 404:
          return UserNotFoundException(message);
        case 409:
          return UserAlreadyExistsException(message);
        case 500:
        case 502:
        case 503:
          return ServerException(message);
        default:
          return UnknownAuthException(message);
      }
    }
    return NetworkException(error.message);
  }
}
