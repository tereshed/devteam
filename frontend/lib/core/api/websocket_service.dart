import 'dart:async';
import 'dart:developer' as developer;
import 'dart:math';

import 'package:clock/clock.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/ws_handshake_unauthorized.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:meta/meta.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

typedef WsChannelFactory = WebSocketChannel Function(
  Uri uri, {
  Iterable<String>? protocols,
});

/// Как закрывать исходящий WebSocket при инициативе клиента (RFC 6455).
enum _WsClientCloseKind {
  /// 1000 — штатное закрытие (disconnect, смена проекта, idle, ошибки кадра).
  normal,

  /// 1001 — уход клиента (pause / dispose).
  goingAway,
}

/// Параметры таймаутов, backoff и circuit breaker (задача 11.2).
@immutable
class WsConfig {
  const WsConfig({
    this.connectTimeout = const Duration(seconds: 10),
    this.idleTimeout = const Duration(seconds: 65),
    this.serverShutdownReconnectDelay = const Duration(seconds: 5),
    this.internalErrorReconnectDelay = const Duration(seconds: 5),
    this.backoffResetAfterStableOpen = const Duration(seconds: 30),
    this.baseMs = 500,
    this.capMs = 30000,
    this.circuitWindow = const Duration(seconds: 10),
    this.circuitParseErrors = 50,
    this.maxConnsReconnectDelay = const Duration(seconds: 60),
  });

  final Duration connectTimeout;
  final Duration idleTimeout;
  final Duration serverShutdownReconnectDelay;
  final Duration internalErrorReconnectDelay;
  final Duration backoffResetAfterStableOpen;
  final int baseMs;
  final int capMs;
  final Duration circuitWindow;
  final int circuitParseErrors;

  /// TODO(7.7): согласовать с бэком (HTTP 429 vs close 4429). Счётчик **4429**
  /// сбрасывается при connect/disconnect, первом envelope и при стабильном open
  /// (см. `WebSocketService`); иначе два **4429** подряд в одном эпизоде → терминал.
  final Duration maxConnsReconnectDelay;

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is WsConfig &&
        other.connectTimeout == connectTimeout &&
        other.idleTimeout == idleTimeout &&
        other.serverShutdownReconnectDelay == serverShutdownReconnectDelay &&
        other.internalErrorReconnectDelay == internalErrorReconnectDelay &&
        other.backoffResetAfterStableOpen == backoffResetAfterStableOpen &&
        other.baseMs == baseMs &&
        other.capMs == capMs &&
        other.circuitWindow == circuitWindow &&
        other.circuitParseErrors == circuitParseErrors &&
        other.maxConnsReconnectDelay == maxConnsReconnectDelay;
  }

  @override
  int get hashCode => Object.hash(
        connectTimeout,
        idleTimeout,
        serverShutdownReconnectDelay,
        internalErrorReconnectDelay,
        backoffResetAfterStableOpen,
        baseMs,
        capMs,
        circuitWindow,
        circuitParseErrors,
        maxConnsReconnectDelay,
      );
}

enum _WsPhase { idle, connecting, open, reconnecting, disposed }

/// Верхняя граница (exclusive) full jitter без overflow double.
int _backoffUpperExclusiveMs(WsConfig c, int attempt) {
  const maxSafePow = 24;
  final exp = min(attempt, maxSafePow);
  final raw = c.baseMs * pow(2, exp);
  final capped = min(c.capMs.toDouble(), raw);
  return max(1, capped.toInt());
}

/// Тесты: формула backoff (задача 11.2).
@visibleForTesting
int backoffUpperExclusiveMsForTesting(WsConfig config, int attempt) =>
    _backoffUpperExclusiveMs(config, attempt);

/// Для UI: обрезка текста reason (не RFC 123 байта UTF-8).
String? _truncatedWsCloseReasonForDisplay(String? reason) {
  if (reason == null || reason.isEmpty) {
    return null;
  }
  const maxChars = 123;
  if (reason.length <= maxChars) {
    return reason;
  }
  return reason.substring(0, maxChars);
}

/// Инфраструктурный WebSocket-клиент: FSM, handshake session, backoff, парсинг 7.3.
///
/// **Стрим [events]:** broadcast без `sync`, слушатели вызываются асинхронно — безопасно
/// вызывать [connect]/[disconnect]/[dispose] из обработчика событий (11.9).
///
/// **Стримы для UI (11.9):** используйте **единый** broadcast-стрим [events].
/// В него попадают:
/// - [WsClientEvent.server] — успешно распарсенные сообщения сервера ([WsServerEvent]);
/// - [WsClientEvent.parseError] — ошибки декодирования кадра ([WsParseError]);
/// - [WsClientEvent.serviceFailure] — терминальные/сетевые сбои сервиса ([WsServiceFailure]);
/// - [WsClientEvent.authFailure] / [WsClientEvent.subprotocolMismatch] — специальные случаи auth/handshake.
///
/// Рекомендуемый паттерн: один `listen` на [events] и `switch` по типу [WsClientEvent].
/// Отдельного стрима `failures` нет — всё объединено в [WsClientEvent] для 11.9.
class WebSocketService {
  WebSocketService({
    required String baseUrl,
    required WsChannelFactory channelFactory,
    required WsAuthProvider authProvider,
    Clock clock = const Clock(),
    WsConfig config = const WsConfig(),
    Random? random,
  })  : _baseUrl = baseUrl,
        _channelFactory = channelFactory,
        _authProvider = authProvider,
        _clock = clock,
        _config = config,
        _random = random ?? Random();

  final String _baseUrl;
  final WsChannelFactory _channelFactory;
  final WsAuthProvider _authProvider;
  final Clock _clock;
  final WsConfig _config;
  final Random _random;

  final StreamController<WsClientEvent> _controller =
      StreamController<WsClientEvent>.broadcast();

  _WsPhase _phase = _WsPhase.idle;
  int _sessionId = 0;
  String? _activeProjectId;
  String? _pausedProjectId;

  bool _manualDisconnect = false;
  bool _paused = false;
  bool _parseCircuitOpen = false;
  bool _terminalPolicy = false;
  bool _terminalAuth = false;

  /// Сколько подряд **4429** без сброса «эпизода» (первый envelope, connect(),
  /// стабильный open ≥ [WsConfig.backoffResetAfterStableOpen], см. обработку в сервисе).
  int _tooManyConns4429Closes = 0;

  int _backoffAttempt = 0;
  final List<DateTime> _parseErrorInstants = <DateTime>[];

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _socketSub;
  Timer? _idleTimer;
  Timer? _backoffTimer;
  Timer? _stableOpenTimer;
  Timer? _oneShotTimer;

  DateTime? _openedAt;
  bool _hasSuccessfulEnvelopeThisOpen = false;
  int? _socketEndCommittedSession;

  /// Единый broadcast-стрим событий (см. dartdoc класса).
  Stream<WsClientEvent> get events => _controller.stream;

  /// Подключение к `/projects/{projectId}/ws`. Возвращает тот же [events] (broadcast, без replay).
  Stream<WsClientEvent> connect(String projectId) {
    _ensureNotDisposed();
    if (_paused) {
      throw StateError('WebSocketService.pause: вызовите resume() перед connect()');
    }
    if (!isValidUuid(projectId)) {
      throw ArgumentError.value(projectId, 'projectId', 'Ожидался UUID проекта');
    }

    if (_phase == _WsPhase.open &&
        _activeProjectId == projectId &&
        _channel != null) {
      return events;
    }

    if ((_phase == _WsPhase.connecting || _phase == _WsPhase.reconnecting) &&
        _activeProjectId == projectId) {
      return events;
    }

    final switchingProject = _phase == _WsPhase.open &&
        _channel != null &&
        _activeProjectId != null &&
        _activeProjectId != projectId;
    if (switchingProject) {
      _emit(
        const WsClientEvent.serviceFailure(
          WsServiceFailure.transient('switching project'),
        ),
      );
    }

    _manualDisconnect = false;
    _terminalPolicy = false;
    _terminalAuth = false;
    _tooManyConns4429Closes = 0;
    _parseCircuitOpen = false;
    _parseErrorInstants.clear();
    _socketEndCommittedSession = null;
    _sessionId++;
    final sid = _sessionId;
    _activeProjectId = projectId;
    _cancelAllTimers();
    _tearDownSocket(kind: _WsClientCloseKind.normal);

    _phase = _WsPhase.connecting;
    _scheduleRunSession(sid, projectId);
    return events;
  }

  void disconnect() {
    if (_phase == _WsPhase.disposed) {
      throw StateError('WebSocketService disposed');
    }
    _paused = false;
    _pausedProjectId = null;
    _manualDisconnect = true;
    _cancelAllTimers();
    _backoffAttempt = 0;
    _parseErrorInstants.clear();
    _tooManyConns4429Closes = 0;
    _socketEndCommittedSession = null;
    _sessionId++;
    _tearDownSocket(kind: _WsClientCloseKind.normal);
    _phase = _WsPhase.idle;
    _activeProjectId = null;
  }

  void pause() {
    _ensureNotDisposed();
    if (_paused) {
      return;
    }
    _paused = true;
    _pausedProjectId = _activeProjectId;
    _manualDisconnect = true;
    _cancelAllTimers();
    _sessionId++;
    _tearDownSocket(kind: _WsClientCloseKind.goingAway);
    _phase = _WsPhase.idle;
  }

  /// Снимает паузу и при наличии сохранённого проекта снова открывает сокет (новый auth внутри connect).
  Future<void> resume() async {
    _ensureNotDisposed();
    if (!_paused) {
      return;
    }
    _paused = false;
    final pid = _pausedProjectId;
    _pausedProjectId = null;
    if (pid != null) {
      connect(pid);
    }
  }

  void dispose() {
    if (_phase == _WsPhase.disposed) {
      return;
    }
    _phase = _WsPhase.disposed;
    _cancelAllTimers();
    _sessionId++;
    _tearDownSocket(kind: _WsClientCloseKind.goingAway);
    unawaited(_controller.close());
  }

  void _ensureNotDisposed() {
    if (_phase == _WsPhase.disposed) {
      throw StateError('WebSocketService disposed');
    }
  }

  void _cancelAllTimers() {
    _idleTimer?.cancel();
    _idleTimer = null;
    _backoffTimer?.cancel();
    _backoffTimer = null;
    _stableOpenTimer?.cancel();
    _stableOpenTimer = null;
    _oneShotTimer?.cancel();
    _oneShotTimer = null;
  }

  void _tearDownSocket({_WsClientCloseKind kind = _WsClientCloseKind.normal}) {
    unawaited(_socketSub?.cancel());
    _socketSub = null;
    final ch = _channel;
    _channel = null;
    if (ch == null) {
      return;
    }
    try {
      switch (kind) {
        case _WsClientCloseKind.normal:
          ch.sink.close(1000, 'client');
          break;
        case _WsClientCloseKind.goingAway:
          ch.sink.close(1001, 'client');
          break;
      }
    } catch (e, st) {
      developer.log(
        'WebSocketService: sink.close',
        name: 'WebSocketService',
        error: e,
        stackTrace: st,
        level: 900,
      );
    }
  }

  void _emit(WsClientEvent e) {
    if (!_controller.isClosed) {
      _controller.add(e);
    }
  }

  void _terminateSubprotocolMismatch(
    WebSocketChannel ch,
    WsAuth auth,
    String? negotiated,
  ) {
    _emit(
      WsClientEvent.subprotocolMismatch(
        WsSubprotocolMismatch(
          expected: wsSubprotocolSchemeExpected(auth),
          received: wsSubprotocolReceivedForDisplay(negotiated),
        ),
      ),
    );
    _emit(
      const WsClientEvent.serviceFailure(
        WsServiceFailure.policySubprotocolMismatch(),
      ),
    );
    _terminalPolicy = true;
    _cancelAllTimers();
    unawaited(ch.sink.close(1000, 'client'));
    _phase = _WsPhase.idle;
  }

  Future<void> _runSession(int session, String projectId) async {
    if (_phase == _WsPhase.disposed) {
      return;
    }
    if (session != _sessionId) {
      return;
    }

    if (_parseCircuitOpen || _terminalPolicy || _terminalAuth) {
      _phase = _WsPhase.idle;
      return;
    }

    _openedAt = null;
    _hasSuccessfulEnvelopeThisOpen = false;
    _socketEndCommittedSession = null;

    final uri = buildProjectWebSocketUri(_baseUrl, projectId);
    WsAuth auth;
    try {
      auth = await _authProvider();
    } catch (e, st) {
      if (session != _sessionId) {
        return;
      }
      developer.log(
        'WebSocketService: authProvider',
        name: 'WebSocketService',
        error: e,
        stackTrace: st,
        level: 900,
      );
      _emit(const WsClientEvent.serviceFailure(WsServiceFailure.transient()));
      _bumpBackoff();
      _scheduleBackoffReconnect(session, projectId);
      return;
    }
    if (session != _sessionId) {
      return;
    }

    final Iterable<String>? protocols;
    if (auth is WsAuthBearer) {
      protocols = <String>['bearer.${auth.jwt}'];
    } else if (auth is WsAuthApiKey) {
      protocols = <String>['apikey.${auth.secret}'];
    } else {
      protocols = null;
    }

    late final WebSocketChannel ch;
    try {
      ch = _channelFactory(uri, protocols: protocols);
    } catch (e) {
      if (session != _sessionId) {
        return;
      }
      _emit(WsClientEvent.serviceFailure(WsServiceFailure.transient(e)));
      _bumpBackoff();
      _scheduleBackoffReconnect(session, projectId);
      return;
    }

    try {
      await ch.ready.timeout(_config.connectTimeout);
    } on TimeoutException {
      if (session != _sessionId) {
        unawaited(ch.sink.close(1000, 'connectTimeout'));
        return;
      }
      _emit(
        WsClientEvent.serviceFailure(
          WsServiceFailure.transient('connectTimeout'),
        ),
      );
      _bumpBackoff();
      unawaited(ch.sink.close(1000, 'connectTimeout'));
      _scheduleBackoffReconnect(session, projectId);
      return;
    } catch (e, st) {
      if (session != _sessionId) {
        unawaited(ch.sink.close(1000, 'client'));
        return;
      }
      developer.log(
        'WebSocketService: ch.ready',
        name: 'WebSocketService',
        error: e,
        stackTrace: st,
        level: 800,
      );
      if (wsHandshakeIndicatesHttpUnauthorized(e)) {
        _cancelAllTimers();
        _emit(const WsClientEvent.serviceFailure(WsServiceFailure.authExpired()));
        _emit(const WsClientEvent.authFailure(WsAuthFailure()));
        _terminalAuth = true;
        _phase = _WsPhase.idle;
      } else {
        _emit(WsClientEvent.serviceFailure(WsServiceFailure.transient()));
        _bumpBackoff();
        _scheduleBackoffReconnect(session, projectId);
      }
      unawaited(ch.sink.close(1000, 'client'));
      return;
    }

    if (session != _sessionId) {
      unawaited(ch.sink.close(1000, 'client'));
      return;
    }

    final negotiated = ch.protocol;
    final expectedWire = switch (auth) {
      WsAuthBearer(:final jwt) => 'bearer.$jwt',
      WsAuthApiKey(:final secret) => 'apikey.$secret',
      WsAuthNone() => null,
      _ => null,
    };
    if (expectedWire != null && negotiated != expectedWire) {
      _terminateSubprotocolMismatch(ch, auth, negotiated);
      return;
    }

    _channel = ch;
    _phase = _WsPhase.open;
    _openedAt = _clock.now();
    _hasSuccessfulEnvelopeThisOpen = false;
    _socketEndCommittedSession = null;
    _armIdleTimer(session, projectId);
    _armStableOpenTimer(session);

    _socketSub = ch.stream.listen(
      (dynamic data) {
        if (session != _sessionId) {
          return;
        }
        if (data is! String) {
          _trackParseError(
            session,
            const WsParseError(message: 'binary frame ignored'),
          );
          return;
        }
        _onFrame(session, projectId, data);
      },
      onError: (Object e, _) {
        if (session != _sessionId) {
          return;
        }
        _handleSocketEnd(
          session,
          projectId,
          closeCode: null,
          streamError: e,
        );
      },
      onDone: () {
        if (session != _sessionId) {
          return;
        }
        _handleSocketEnd(
          session,
          projectId,
          closeCode: ch.closeCode,
          closeReason: ch.closeReason,
        );
      },
      cancelOnError: true,
    );
  }

  void _onFrame(int session, String projectId, String data) {
    if (session != _sessionId) {
      return;
    }
    if (_terminalPolicy || _terminalAuth || _parseCircuitOpen) {
      return;
    }
    _armIdleTimer(session, projectId);
    try {
      final ev = parseWsServerEnvelope(data);
      if (!_hasSuccessfulEnvelopeThisOpen) {
        _hasSuccessfulEnvelopeThisOpen = true;
        _backoffAttempt = 0;
        _tooManyConns4429Closes = 0;
      }
      _emit(WsClientEvent.server(ev));
      _applyErrorCodeSideEffects(session, projectId, ev);
    } on WsParseError catch (e) {
      _trackParseError(session, e);
    } catch (e) {
      _trackParseError(session, WsParseError(message: '$e'));
    }
  }

  void _trackParseError(int session, WsParseError e) {
    if (session != _sessionId) {
      return;
    }
    if (_parseCircuitOpen) {
      return;
    }
    if (_terminalPolicy || _terminalAuth) {
      return;
    }
    final now = _clock.now();
    _parseErrorInstants.removeWhere(
      (t) => now.difference(t) > _config.circuitWindow,
    );
    _parseErrorInstants.add(now);
    _emit(WsClientEvent.parseError(e));
    if (_parseErrorInstants.length >= _config.circuitParseErrors) {
      _parseCircuitOpen = true;
      _emit(
        const WsClientEvent.serviceFailure(WsServiceFailure.protocolBroken()),
      );
      _sessionId++;
      _tearDownSocket(kind: _WsClientCloseKind.normal);
      _phase = _WsPhase.idle;
    }
  }

  void _applyErrorCodeSideEffects(int session, String projectId, WsServerEvent ev) {
    ev.maybeWhen(
      error: (WsErrorEvent err) {
        switch (err.code) {
          case WsErrorCode.forbidden:
            _cancelAllTimers();
            _terminalPolicy = true;
            _emit(
              const WsClientEvent.serviceFailure(
                WsServiceFailure.policyForbidden(),
              ),
            );
            _sessionId++;
            _tearDownSocket(kind: _WsClientCloseKind.normal);
            _phase = _WsPhase.idle;
            return;
          case WsErrorCode.serverShutdown:
            // Инкремент сессии до tearDown: onDone старого канала отсекается по session,
            // не планирует второй reconnect поверх one-shot anti-herd.
            _idleTimer?.cancel();
            _stableOpenTimer?.cancel();
            _sessionId++;
            final sid = _sessionId;
            _tearDownSocket(kind: _WsClientCloseKind.normal);
            _phase = _WsPhase.reconnecting;
            _scheduleServerShutdown(sid, projectId);
            return;
          case WsErrorCode.internalError:
            _idleTimer?.cancel();
            _stableOpenTimer?.cancel();
            _sessionId++;
            final sid = _sessionId;
            _tearDownSocket(kind: _WsClientCloseKind.normal);
            _phase = _WsPhase.reconnecting;
            _scheduleOneShot(sid, projectId, _config.internalErrorReconnectDelay);
            return;
          case WsErrorCode.streamOverflow:
          case WsErrorCode.taskNotFound:
            return;
        }
      },
      orElse: () {},
    );
  }

  void _scheduleServerShutdown(int session, String projectId) {
    final base = _config.serverShutdownReconnectDelay;
    final jitterSmall = _random.nextInt(501);
    final herd = _random.nextInt(8001) + 2000;
    final delayMs = base.inMilliseconds + jitterSmall + herd;
    _scheduleOneShotMs(session, projectId, delayMs);
  }

  void _scheduleOneShot(int session, String projectId, Duration d) {
    _scheduleOneShotMs(session, projectId, d.inMilliseconds);
  }

  void _scheduleOneShotMs(int session, String projectId, int delayMs) {
    _oneShotTimer?.cancel();
    _oneShotTimer = Timer(Duration(milliseconds: delayMs), () {
      if (_phase == _WsPhase.disposed) {
        return;
      }
      if (_manualDisconnect || _paused || _parseCircuitOpen) {
        return;
      }
      if (_terminalPolicy || _terminalAuth) {
        return;
      }
      if (session != _sessionId) {
        return;
      }
      _kickConnectSession(projectId);
    });
  }

  void _handleSocketEnd(
    int session,
    String projectId, {
    int? closeCode,
    String? closeReason,
    Object? streamError,
  }) {
    if (_socketEndCommittedSession == session) {
      return;
    }

    _idleTimer?.cancel();
    _stableOpenTimer?.cancel();
    unawaited(_socketSub?.cancel());
    _socketSub = null;
    _channel = null;

    if (_phase == _WsPhase.disposed) {
      return;
    }
    if (session != _sessionId) {
      return;
    }

    _socketEndCommittedSession = session;

    if (_manualDisconnect || _paused) {
      _phase = _WsPhase.idle;
      return;
    }

    if (_parseCircuitOpen || _terminalPolicy || _terminalAuth) {
      _phase = _WsPhase.idle;
      return;
    }

    if (streamError != null) {
      _emit(WsClientEvent.serviceFailure(WsServiceFailure.transient(streamError)));
    }

    final code = closeCode ?? 1006;

    if (code == 4401) {
      _cancelAllTimers();
      _terminalAuth = true;
      _emit(const WsClientEvent.serviceFailure(WsServiceFailure.authExpired()));
      _emit(
        WsClientEvent.authFailure(
          WsAuthFailure(
            closeCode: code,
            closeReason: _truncatedWsCloseReasonForDisplay(closeReason),
          ),
        ),
      );
      _phase = _WsPhase.idle;
      return;
    }
    if (code == 4429) {
      final opened = _openedAt;
      if (opened != null &&
          _clock.now().difference(opened) >=
              _config.backoffResetAfterStableOpen) {
        _tooManyConns4429Closes = 0;
      }
      _tooManyConns4429Closes++;
      if (_tooManyConns4429Closes > 1) {
        _cancelAllTimers();
        _terminalPolicy = true;
        _emit(
          const WsClientEvent.serviceFailure(
            WsServiceFailure.tooManyConnectionsTerminal(),
          ),
        );
        _phase = _WsPhase.idle;
        return;
      }
      _emit(
        const WsClientEvent.serviceFailure(WsServiceFailure.tooManyConnections()),
      );
      // TODO(7.7): согласовать 4429/429 с бэком. Сброс счётчика: connect/disconnect,
      // первый envelope, стабильный open (см. выше и _maybeResetAttemptFromStableOpen).
      _phase = _WsPhase.reconnecting;
      _scheduleOneShot(session, projectId, _config.maxConnsReconnectDelay);
      return;
    }
    if (code == 1008) {
      _cancelAllTimers();
      _terminalPolicy = true;
      _emit(
        WsClientEvent.serviceFailure(
          WsServiceFailure.policyCloseCode(code),
        ),
      );
      _phase = _WsPhase.idle;
      return;
    }

    if (code == 1000 || code == 1001 || code == 1006 || code == 1011) {
      final reset = _maybeResetAttemptFromStableOpen();
      if (!reset) {
        _bumpBackoff();
      }
      _scheduleBackoffReconnect(session, projectId);
      return;
    }

    _emit(WsClientEvent.serviceFailure(WsServiceFailure.transient('close $code')));
    _bumpBackoff();
    _scheduleBackoffReconnect(session, projectId);
  }

  bool _maybeResetAttemptFromStableOpen() {
    final opened = _openedAt;
    if (opened != null &&
        _clock.now().difference(opened) >= _config.backoffResetAfterStableOpen) {
      _backoffAttempt = 0;
      _tooManyConns4429Closes = 0;
      return true;
    }
    return false;
  }

  void _bumpBackoff() {
    _backoffAttempt = min(_backoffAttempt + 1, 100);
  }

  void _scheduleBackoffReconnect(int session, String projectId) {
    if (_manualDisconnect || _paused || _parseCircuitOpen) {
      return;
    }
    if (_terminalPolicy || _terminalAuth) {
      return;
    }
    if (_phase == _WsPhase.disposed) {
      return;
    }

    final upperExclusive = _backoffUpperExclusiveMs(_config, _backoffAttempt);
    final delayMs = _random.nextInt(upperExclusive);

    _phase = _WsPhase.reconnecting;
    _backoffTimer?.cancel();
    _backoffTimer = Timer(Duration(milliseconds: delayMs), () {
      if (_phase == _WsPhase.disposed) {
        return;
      }
      if (session != _sessionId) {
        return;
      }
      if (_manualDisconnect || _paused || _parseCircuitOpen) {
        return;
      }
      if (_terminalPolicy || _terminalAuth) {
        return;
      }
      _kickConnectSession(projectId);
    });
  }

  void _scheduleRunSession(int sessionId, String projectId) {
    _phase = _WsPhase.connecting;
    scheduleMicrotask(() => _runSession(sessionId, projectId));
  }

  void _kickConnectSession(String projectId) {
    _sessionId++;
    _scheduleRunSession(_sessionId, projectId);
  }

  void _armIdleTimer(int session, String projectId) {
    _idleTimer?.cancel();
    _idleTimer = Timer(_config.idleTimeout, () {
      if (session != _sessionId) {
        return;
      }
      _emit(
        WsClientEvent.serviceFailure(
          WsServiceFailure.transient('idleTimeout'),
        ),
      );
      unawaited(_socketSub?.cancel());
      _socketSub = null;
      try {
        _channel?.sink.close(1000, 'idleTimeout');
      } catch (e, st) {
        developer.log(
          'WebSocketService: idle sink.close',
          name: 'WebSocketService',
          error: e,
          stackTrace: st,
          level: 900,
        );
      }
      _channel = null;
      _handleSocketEnd(session, projectId, closeCode: 1006);
    });
  }

  void _armStableOpenTimer(int session) {
    _stableOpenTimer?.cancel();
    _stableOpenTimer = Timer(_config.backoffResetAfterStableOpen, () {
      if (session != _sessionId) {
        return;
      }
      if (_phase != _WsPhase.open) {
        return;
      }
      _backoffAttempt = 0;
    });
  }
}
