/// На web нет типизированного HTTP-кода handshake — [wsHandshakeIndicatesHttpUnauthorized]
/// всегда false; истёкший JWT ожидаем по close **4401** после успешного open (см. задачу 11.2).
///
/// TODO(11.2): при появлении стабильного сигнала HTTP 401 на web — расширить stub / тесты.
bool wsHandshakeIndicatesHttpUnauthorized(Object error) => false;
