abstract class AuthException implements Exception {
  final String? message;
  const AuthException([this.message]);
}

class InvalidCredentialsException extends AuthException {
  const InvalidCredentialsException([super.message]);
}

class UserNotFoundException extends AuthException {
  const UserNotFoundException([super.message]);
}

class UserAlreadyExistsException extends AuthException {
  const UserAlreadyExistsException([super.message]);
}

class AccessDeniedException extends AuthException {
  const AccessDeniedException([super.message]);
}

class NetworkException extends AuthException {
  const NetworkException([super.message]);
}

class ServerException extends AuthException {
  const ServerException([super.message]);
}

class UnknownAuthException extends AuthException {
  const UnknownAuthException([super.message]);
}
