/// CommonMark / GFM: fence до трёх пробелов отступа; 4+ пробела — это уже не fence (индентированный код),
/// строка с литеральными ``` — не закрытие.
///
/// См. раздел Fenced code blocks в CommonMark.
bool isBacktickFenceLine(String line) => RegExp(r'^ {0,3}`{3,}').hasMatch(line);

bool isTildeFenceLine(String line) => RegExp(r'^ {0,3}~{3,}').hasMatch(line);

/// Если fence ``` или ~~~ не закрыт — добавляем закрывающую строку в конец (стрим ассистента).
///
/// Поддерживаются оба вида GFM fence (backtick и tilde).
String preprocessUnclosedFence(String markdown, {required bool isStreaming}) {
  if (!isStreaming) {
    return markdown;
  }
  final lines = markdown.split('\n');
  var inBacktickFence = false;
  var inTildeFence = false;

  for (final line in lines) {
    if (inBacktickFence) {
      if (isBacktickFenceLine(line)) {
        inBacktickFence = false;
      }
    } else if (inTildeFence) {
      if (isTildeFenceLine(line)) {
        inTildeFence = false;
      }
    } else {
      if (isBacktickFenceLine(line)) {
        inBacktickFence = true;
      } else if (isTildeFenceLine(line)) {
        inTildeFence = true;
      }
    }
  }

  if (inBacktickFence) {
    return '$markdown\n```';
  }
  if (inTildeFence) {
    return '$markdown\n~~~';
  }
  return markdown;
}
