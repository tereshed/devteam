// Unified diff parsing for DiffViewer (12.7).
// Kept next to the widget in shared — not in features/tasks.

/// Класс строки diff после разбора.
enum DiffParsedLineKind {
  /// Весь текст как plain monospace (нет признаков unified diff).
  plainBlock,

  /// Метаданные (index, rename, binary, submodule, \ No newline, …).
  metadata,

  /// Заголовки `--- a/…` / `+++ b/…` вне hunk (или смена файла).
  fileHeader,

  /// Строка `@@ … @@`.
  hunkHeader,

  /// Контекст внутри hunk (префикс пробел).
  context,

  /// Добавленная строка внутри hunk.
  addition,

  /// Удалённая строка внутри hunk.
  deletion,
}

/// Одна логическая строка для рендера (без завершающего `\n`).
class DiffParsedLine {
  const DiffParsedLine(this.text, this.kind);

  final String text;
  final DiffParsedLineKind kind;
}

bool diffLooksLikeUnified(List<String> lines) {
  var hasHunk = false;
  var hasGit = false;
  var hasMinus = false;
  var hasPlus = false;
  for (final line in lines) {
    if (_isHunkHeaderLine(line)) {
      hasHunk = true;
    }
    if (line.startsWith('diff --git')) {
      hasGit = true;
    }
    if (line.startsWith('--- ')) {
      hasMinus = true;
    }
    if (line.startsWith('+++ ')) {
      hasPlus = true;
    }
  }
  return hasHunk || hasGit || (hasMinus && hasPlus);
}

bool _isHunkHeaderLine(String line) {
  if (!line.startsWith('@@')) {
    return false;
  }
  return line.lastIndexOf('@@') > 1;
}

bool _isFileHeaderMinus(String line) => line.startsWith('--- ');

bool _isFileHeaderPlus(String line) => line.startsWith('+++ ');

bool _isNoNewlineGitLine(String line) =>
    line.startsWith(r'\') && line.contains('No newline at end of file');

bool _isMetadataPattern(String line) {
  if (line.startsWith('diff --git')) {
    return true;
  }
  if (line.startsWith('index ')) {
    return true;
  }
  if (line.startsWith('similarity index')) {
    return true;
  }
  if (line.startsWith('rename from')) {
    return true;
  }
  if (line.startsWith('rename to')) {
    return true;
  }
  if (line.startsWith('Binary files ') && line.contains(' differ')) {
    return true;
  }
  if (line.startsWith('Submodule ')) {
    return true;
  }
  if (line.startsWith('Subproject commit ')) {
    return true;
  }
  if (_isNoNewlineGitLine(line)) {
    return true;
  }
  return false;
}

/// Парсинг unified diff без выбросов; [raw] не триммится.
List<DiffParsedLine> parseUnifiedDiff(String raw) {
  final lines = raw.split('\n');
  if (!diffLooksLikeUnified(lines)) {
    return [DiffParsedLine(raw, DiffParsedLineKind.plainBlock)];
  }

  final out = <DiffParsedLine>[];
  var inHunk = false;

  for (final line in lines) {
    // Внутри hunk: выход только через `diff --git` или новый `@@`.
    // `--- a/`/`+++ b/` НЕ переключают режим — спорная строка содержимого
    // (§ Состояние парсера п. 3) важнее, чем мульти-файл без `diff --git`.
    if (inHunk) {
      if (line.startsWith('diff --git')) {
        inHunk = false;
        out.add(DiffParsedLine(line, DiffParsedLineKind.metadata));
        continue;
      }
      if (_isHunkHeaderLine(line)) {
        out.add(DiffParsedLine(line, DiffParsedLineKind.hunkHeader));
        continue;
      }
      if (_isNoNewlineGitLine(line)) {
        out.add(DiffParsedLine(line, DiffParsedLineKind.metadata));
        continue;
      }
      if (line.isEmpty) {
        out.add(DiffParsedLine(line, DiffParsedLineKind.context));
        continue;
      }
      final c0 = line.codeUnitAt(0);
      if (c0 == 0x20) {
        out.add(DiffParsedLine(line, DiffParsedLineKind.context));
      } else if (c0 == 0x2B) {
        out.add(DiffParsedLine(line, DiffParsedLineKind.addition));
      } else if (c0 == 0x2D) {
        out.add(DiffParsedLine(line, DiffParsedLineKind.deletion));
      } else {
        // Не префикс hunk (' ' / '+' / '-') — консервативно metadata.
        out.add(DiffParsedLine(line, DiffParsedLineKind.metadata));
      }
      continue;
    }

    // Вне hunk
    if (_isHunkHeaderLine(line)) {
      out.add(DiffParsedLine(line, DiffParsedLineKind.hunkHeader));
      inHunk = true;
      continue;
    }
    if (_isFileHeaderMinus(line) || _isFileHeaderPlus(line)) {
      out.add(DiffParsedLine(line, DiffParsedLineKind.fileHeader));
      continue;
    }
    if (_isMetadataPattern(line)) {
      out.add(DiffParsedLine(line, DiffParsedLineKind.metadata));
      continue;
    }
    // Вне hunk: строки без отдельного паттерна — metadata (§ Edge-cases).
    out.add(DiffParsedLine(line, DiffParsedLineKind.metadata));
  }

  return out;
}
