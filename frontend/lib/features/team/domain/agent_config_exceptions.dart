import 'package:dio/dio.dart';

class AgentConfigApiException implements Exception {
  AgentConfigApiException(this.message, {this.statusCode, this.originalError, this.apiErrorCode});
  final String message;
  final int? statusCode;
  final DioException? originalError;
  final String? apiErrorCode;
  @override
  String toString() => 'AgentConfigApiException($statusCode): $message';
}

class AgentConfigCancelledException implements Exception {
  AgentConfigCancelledException(this.message, {this.originalError});
  final String message;
  final DioException? originalError;
}

class AgentConfigNotFoundException implements Exception {
  AgentConfigNotFoundException(this.message, {this.originalError, this.apiErrorCode});
  final String message;
  final DioException? originalError;
  final String? apiErrorCode;
}

class AgentConfigForbiddenException implements Exception {
  AgentConfigForbiddenException(this.message, {this.originalError, this.apiErrorCode});
  final String message;
  final DioException? originalError;
  final String? apiErrorCode;
}

class AgentConfigConflictException implements Exception {
  AgentConfigConflictException(this.message, {this.originalError, this.apiErrorCode});
  final String message;
  final DioException? originalError;
  final String? apiErrorCode;
}
