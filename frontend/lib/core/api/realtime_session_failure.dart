import 'package:freezed_annotation/freezed_annotation.dart';

part 'realtime_session_failure.freezed.dart';

/// Терминальный сбой realtime-сессии (JWT / политика / лимит коннектов).
///
/// Нейтральный тип для `core/api` — без зависимости от features; чат и задачи могут
/// хранить его в state или маппить в UI-специфичные обёртки.
@freezed
abstract class RealtimeSessionFailure with _$RealtimeSessionFailure {
  const factory RealtimeSessionFailure.authenticationLost() =
      _RealtimeSessionFailureAuth;
  const factory RealtimeSessionFailure.connectionLimitExceeded() =
      _RealtimeSessionFailureConnLimit;
  const factory RealtimeSessionFailure.forbidden() =
      _RealtimeSessionFailureForbidden;
}
