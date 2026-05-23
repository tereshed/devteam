import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/features/assistant/domain/assistant_session_model.dart';
import 'package:frontend/features/assistant/presentation/controllers/assistant_chat_controller.dart';
import 'package:frontend/features/projects/data/project_providers.dart';

/// Dropdown в header'е панели чата: текущая сессия + список последних сессий
/// пользователя (Sprint 21 §10 frontend).
///
/// Список тянем через `assistantSessionsListProvider` (autoDispose), чтобы
/// обновлять при открытии меню без ручного refresh.
class AssistantSessionPicker extends ConsumerWidget {
  const AssistantSessionPicker({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(
      context,
      where: 'AssistantSessionPicker',
    );
    final theme = Theme.of(context);
    final chatState = ref.watch(assistantChatControllerProvider);
    final current = chatState.session;
    final asyncSessions = ref.watch(assistantSessionsListProvider);

    final currentTitle = current?.title?.trim().isNotEmpty == true
        ? current!.title!
        : (current != null ? l10n.assistantSessionUntitled : '—');

    return PopupMenuButton<String>(
      tooltip: l10n.assistantSidebarTitle,
      onSelected: (id) {
        ref.read(assistantChatControllerProvider.notifier).selectSession(id);
      },
      itemBuilder: (context) {
        return asyncSessions.maybeWhen(
          data: (sessions) {
            if (sessions.isEmpty) {
              return [
                PopupMenuItem<String>(
                  enabled: false,
                  child: Text(
                    l10n.assistantSessionUntitled,
                    style: theme.textTheme.bodySmall,
                  ),
                ),
              ];
            }
            return [
              for (final s in sessions)
                PopupMenuItem<String>(
                  value: s.id,
                  child: _SessionPickerItem(session: s),
                ),
            ];
          },
          orElse: () => [
            const PopupMenuItem<String>(
              enabled: false,
              child: SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            ),
          ],
        );
      },
      child: Row(
        children: [
          Expanded(
            child: Text(
              currentTitle,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
          const Icon(Icons.arrow_drop_down),
        ],
      ),
    );
  }
}

class _SessionPickerItem extends StatelessWidget {
  const _SessionPickerItem({required this.session});

  final AssistantSessionModel session;

  @override
  Widget build(BuildContext context) {
    final l10n =
        requireAppLocalizations(context, where: '_SessionPickerItem');
    final theme = Theme.of(context);
    final title = session.title?.trim().isNotEmpty == true
        ? session.title!
        : l10n.assistantSessionUntitled;
    return SizedBox(
      width: 260,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: theme.textTheme.bodyMedium,
          ),
          if (session.lastMessageAt != null)
            Text(
              session.lastMessageAt!.toLocal().toIso8601String(),
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
        ],
      ),
    );
  }
}

/// Список сессий пользователя для PopupMenu. autoDispose — снимаем подписку,
/// когда picker свёрнут, чтобы не держать сетевые ресурсы.
///
/// Декларация — обычный [FutureProvider] (не codegen), потому что иначе
/// фича получает «лишний» .g.dart только под один тривиальный future.
final assistantSessionsListProvider =
    FutureProvider.autoDispose<List<AssistantSessionModel>>((ref) async {
  final repo = ref.watch(assistantRepositoryProvider);
  final projectId = ref.watch(activeProjectIdProvider);
  final resp = await repo.listSessions(projectId: projectId);
  return resp.sessions;
});
