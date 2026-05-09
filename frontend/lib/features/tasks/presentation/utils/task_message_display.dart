import 'package:frontend/l10n/app_localizations.dart';

/// Единственная точка маппинга `message_type` → l10n (12.5, review.md DRY).
String taskMessageTypeLabel(AppLocalizations l10n, String messageType) {
  return switch (messageType) {
    'instruction' => l10n.taskMessageTypeInstruction,
    'result' => l10n.taskMessageTypeResult,
    'question' => l10n.taskMessageTypeQuestion,
    'feedback' => l10n.taskMessageTypeFeedback,
    'error' => l10n.taskMessageTypeError,
    'comment' => l10n.taskMessageTypeComment,
    'summary' => l10n.taskMessageTypeSummary,
    _ => l10n.taskMessageTypeUnknown,
  };
}

/// Единственная точка маппинга `sender_type` → l10n (12.5).
String taskSenderTypeLabel(AppLocalizations l10n, String senderType) {
  return switch (senderType) {
    'user' => l10n.taskSenderTypeUser,
    'agent' => l10n.taskSenderTypeAgent,
    _ => l10n.taskSenderTypeUnknown,
  };
}
