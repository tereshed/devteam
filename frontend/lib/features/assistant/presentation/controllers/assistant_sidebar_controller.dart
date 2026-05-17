import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'assistant_sidebar_controller.g.dart';

/// Текущая вкладка правой панели.
enum AssistantSidebarTab {
  /// Чат с ассистентом.
  chat,

  /// Активные задачи всех проектов пользователя.
  tasks,
}

/// UI-состояние правой панели: открыта/закрыта + активная вкладка.
///
/// Сама панель в layout AppShell живёт всегда (для desktop), а флаг [open]
/// управляет показом панели/иконкой-toggle. На tablet/mobile (см.
/// `responsive.dart`) — open=true означает «развернуть как Drawer end-side».
class AssistantSidebarState {
  const AssistantSidebarState({
    this.open = true,
    this.tab = AssistantSidebarTab.chat,
  });

  /// `true` — панель развёрнута, видна. По умолчанию открыта (план §2 frontend:
  /// «Desktop: всегда виден ... по умолчанию open»).
  final bool open;

  /// Активная вкладка.
  final AssistantSidebarTab tab;

  AssistantSidebarState copyWith({
    bool? open,
    AssistantSidebarTab? tab,
  }) {
    return AssistantSidebarState(
      open: open ?? this.open,
      tab: tab ?? this.tab,
    );
  }
}

@Riverpod(keepAlive: true)
class AssistantSidebarController extends _$AssistantSidebarController {
  @override
  AssistantSidebarState build() => const AssistantSidebarState();

  /// Toggle открытой/закрытой панели (binding на AppBar IconButton).
  void toggleOpen() {
    state = state.copyWith(open: !state.open);
  }

  void setOpen(bool value) {
    if (state.open == value) return;
    state = state.copyWith(open: value);
  }

  void setTab(AssistantSidebarTab tab) {
    if (state.tab == tab) return;
    state = state.copyWith(tab: tab);
  }
}
