package sandbox

// tee создаёт n каналов-дублёров из одного источника.
// Все получатели читают из своих каналов; закрытие src закрывает все.
// При переполнении любого из выходных каналов запись в него пропускается (slow consumer).
func tee(src <-chan LogEntry, n int) []<-chan LogEntry {
	chs := make([]<-chan LogEntry, n)
	out := make([]chan LogEntry, n)
	for i := 0; i < n; i++ {
		// Используем буферизацию как в источнике для минимизации блокировок
		out[i] = make(chan LogEntry, StreamLogsDefaultBuffer)
		chs[i] = out[i]
	}

	go func() {
		defer func() {
			for i := 0; i < n; i++ {
				close(out[i])
			}
		}()

		for entry := range src {
			for i := 0; i < n; i++ {
				select {
				case out[i] <- entry:
				default:
					// Получатель не успевает — пропускаем для этого конкретного канала
				}
			}
		}
	}()

	return chs
}
