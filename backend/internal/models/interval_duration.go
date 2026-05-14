package models

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// IntervalDuration — sql.Scanner + driver.Valuer для маппинга PostgreSQL INTERVAL
// ↔ Go time.Duration.
//
// Зачем отдельный тип: либ-стандартный database/sql/driver не знает как
// сериализовать time.Duration в INTERVAL; GORM с pgx может возвращать INTERVAL
// как string "01:00:00", "1 day 02:30:00", "30:00" — единого формата нет.
//
// Контракт:
//   - Сериализация (Value): "%d microseconds" — это валидный INTERVAL литерал в PG,
//     точность достаточная для секундных таймаутов, и Yugabyte интерпретирует одинаково.
//   - Парсинг (Scan): принимает string/[]byte в любом из распространённых форматов:
//     "HH:MM:SS[.ffffff]", "D days HH:MM:SS", "N microseconds", "N seconds".
//   - NULL → IntervalDuration(0).
//
// Использование в моделях: `*IntervalDuration` (nullable) или `IntervalDuration` (с default 0).
type IntervalDuration time.Duration

// Duration возвращает значение как обычный time.Duration.
func (i IntervalDuration) Duration() time.Duration {
	return time.Duration(i)
}

// Value реализует driver.Valuer. Формат "%d microseconds" — PG INTERVAL-литерал.
func (i IntervalDuration) Value() (driver.Value, error) {
	micros := time.Duration(i).Microseconds()
	return fmt.Sprintf("%d microseconds", micros), nil
}

// Scan реализует sql.Scanner.
//
// PostgreSQL gomain выдаёт INTERVAL обычно как `HH:MM:SS` (для < 1 day) или
// `<N> days HH:MM:SS.ffffff`. Yugabyte ведёт себя идентично.
// pgx может также возвращать pgtype.Interval — поддерживаем через ToDuration метод,
// но через рефлексию, чтобы не тянуть pgx как dependency в models.
func (i *IntervalDuration) Scan(value any) error {
	if value == nil {
		*i = 0
		return nil
	}

	var s string
	switch v := value.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		// Поддержим time.Duration напрямую если драйвер уже сконвертировал
		// (некоторые драйверы умеют).
		if d, ok := value.(time.Duration); ok {
			*i = IntervalDuration(d)
			return nil
		}
		return fmt.Errorf("IntervalDuration.Scan: unsupported source type %T", value)
	}

	d, err := parsePGInterval(s)
	if err != nil {
		return fmt.Errorf("IntervalDuration.Scan: %w", err)
	}
	*i = IntervalDuration(d)
	return nil
}

// parsePGInterval разбирает строковые представления INTERVAL.
// Поддерживаемые формы:
//   "HH:MM:SS"               — менее суток
//   "HH:MM:SS.ffffff"        — с микросекундами
//   "D days HH:MM:SS"        — суточные
//   "D days HH:MM:SS.ffffff"
//   "N microseconds" / "N seconds" / "N minutes" / "N hours" — наш Value-формат и эквиваленты.
func parsePGInterval(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// Форма "<N> <unit>" (наш Value-формат, и совместимая с PG-литералами).
	// Учитываем единичные и множественные формы.
	if d, ok := tryParseNumberUnit(s); ok {
		return d, nil
	}

	// Формы с "days".
	var days int64
	rest := s
	if idx := strings.Index(s, "days"); idx > 0 {
		daysStr := strings.TrimSpace(s[:idx])
		var err error
		days, err = strconv.ParseInt(daysStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid days component %q: %w", daysStr, err)
		}
		rest = strings.TrimSpace(s[idx+len("days"):])
	} else if idx := strings.Index(s, "day"); idx > 0 { // "1 day"
		daysStr := strings.TrimSpace(s[:idx])
		var err error
		days, err = strconv.ParseInt(daysStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid day component %q: %w", daysStr, err)
		}
		rest = strings.TrimSpace(s[idx+len("day"):])
	}

	hms := time.Duration(0)
	if rest != "" {
		var err error
		hms, err = parseHMS(rest)
		if err != nil {
			return 0, err
		}
	}
	return time.Duration(days)*24*time.Hour + hms, nil
}

// parseHMS разбирает "HH:MM:SS" или "HH:MM:SS.ffffff".
func parseHMS(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("expected HH:MM:SS, got %q", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid hours %q: %w", parts[0], err)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minutes %q: %w", parts[1], err)
	}
	// Секунды могут быть с дробной частью "SS.ffffff".
	secStr := parts[2]
	micro := int64(0)
	if dot := strings.Index(secStr, "."); dot >= 0 {
		fracStr := secStr[dot+1:]
		// Дополняем до 6 цифр (микросекунды).
		if len(fracStr) > 6 {
			fracStr = fracStr[:6]
		}
		for len(fracStr) < 6 {
			fracStr += "0"
		}
		var err error
		micro, err = strconv.ParseInt(fracStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid fractional seconds %q: %w", parts[2], err)
		}
		secStr = secStr[:dot]
	}
	sec, err := strconv.Atoi(secStr)
	if err != nil {
		return 0, fmt.Errorf("invalid seconds %q: %w", parts[2], err)
	}
	return time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second +
		time.Duration(micro)*time.Microsecond, nil
}

// tryParseNumberUnit пытается распознать "<N> <unit>" формат.
func tryParseNumberUnit(s string) (time.Duration, bool) {
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return 0, false
	}
	n, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	unit := strings.ToLower(parts[1])
	unit = strings.TrimSuffix(unit, "s") // microseconds → microsecond и т.д.
	switch unit {
	case "microsecond":
		return time.Duration(n) * time.Microsecond, true
	case "millisecond":
		return time.Duration(n) * time.Millisecond, true
	case "second", "sec":
		return time.Duration(n) * time.Second, true
	case "minute", "min":
		return time.Duration(n) * time.Minute, true
	case "hour":
		return time.Duration(n) * time.Hour, true
	}
	return 0, false
}
