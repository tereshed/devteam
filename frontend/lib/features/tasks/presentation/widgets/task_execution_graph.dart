import 'dart:math';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/features/tasks/domain/models/task_event_model.dart';
import 'package:intl/intl.dart';

enum NodeStatus { pending, running, success, failed }

/// Тип ноды графа: исполнитель (agent), решение Router'а (router) или корень-оркестратор
/// (orchestrator). Router и orchestrator делают видимым сам процесс принятия решений,
/// раньше «спрятанный» в рёбрах между шагами.
enum NodeKind { agent, router, orchestrator }

class AgentNodeData {
  final String id;
  final String name;
  final String role;
  final NodeStatus status;
  final List<String> subtasks;
  final List<Artifact> artifacts;
  final int stepNo;
  final NodeKind kind;
  final String? reason; // для router-ноды — reason решения
  final String? instructions;
  final List<String>? targetArtifactIds;

  const AgentNodeData({
    required this.id,
    required this.name,
    required this.role,
    required this.status,
    required this.subtasks,
    required this.artifacts,
    required this.stepNo,
    this.kind = NodeKind.agent,
    this.reason,
    this.instructions,
    this.targetArtifactIds,
  });
}

List<AgentNodeData> buildAgentNodes({
  required List<RouterDecision> decisions,
  required List<Artifact> artifacts,
  required List<TaskEventModel> events,
  required String taskState,
  required String? assignedAgentName,
  required String? assignedAgentRole,
  required List<AgentModel> teamAgents,
}) {
  final nodes = <AgentNodeData>[];

  // Сортировка по stepNo, с тай-брейком по createdAt: step_no обычно уникален, но если
  // в данных встретятся дубликаты (исторические задачи до фикса инкремента), тай-брейк
  // по времени держит окна привязки артефактов монотонными и не даёт одному артефакту
  // попасть на две ноды (перекрытие окон).
  final rawSortedDecisions = List<RouterDecision>.from(decisions)
    ..sort((a, b) {
      final byStep = a.stepNo.compareTo(b.stepNo);
      return byStep != 0 ? byStep : a.createdAt.compareTo(b.createdAt);
    });
  // Дедуп по step_no: x-колонка = stepNo*2, поэтому дубли (легаси-задачи до фикса
  // resume, переиспользовавшего номера) накладывались в одну колонку. Оставляем
  // первое (раннее) решение на step_no.
  final sortedDecisions = <RouterDecision>[];
  {
    final seenSteps = <int>{};
    for (final d in rawSortedDecisions) {
      if (seenSteps.add(d.stepNo)) {
        sortedDecisions.add(d);
      }
    }
  }

  if (sortedDecisions.isEmpty) {
    if (assignedAgentName != null) {
      final role = teamAgents.firstWhere(
        (a) => a.name == assignedAgentName,
        orElse: () => AgentModel(id: assignedAgentName, name: assignedAgentName, role: assignedAgentRole ?? assignedAgentName, isActive: true),
      ).role;
      nodes.add(AgentNodeData(
        id: "0_${assignedAgentName}_0",
        name: assignedAgentName,
        role: role,
        status: taskState == 'active' ? NodeStatus.running : NodeStatus.pending,
        subtasks: const [],
        artifacts: const [],
        stepNo: 0,
      ));
    } else {
      for (int i = 0; i < teamAgents.length; i++) {
        final a = teamAgents[i];
        nodes.add(AgentNodeData(
          id: "0_${a.name}_0",
          name: a.name,
          role: a.role,
          status: NodeStatus.pending,
          subtasks: const [],
          artifacts: const [],
          stepNo: 0,
        ));
      }
    }
    return nodes;
  }

  for (int i = 0; i < sortedDecisions.length; i++) {
    final d = sortedDecisions[i];
    final isLast = i == sortedDecisions.length - 1;
    final isStepRunning = !d.done && taskState == 'active';

    final start = d.createdAt;
    final end = !isLast ? sortedDecisions[i + 1].createdAt : DateTime.now().add(const Duration(days: 1));

    // Router-нода шага: делает видимым само решение (reason/outcome) и ветвление в агентов.
    NodeStatus routerStatus;
    if (d.outcome == 'failed' || d.outcome == 'needs_human') {
      routerStatus = NodeStatus.failed;
    } else if (isStepRunning && isLast) {
      routerStatus = NodeStatus.running;
    } else {
      routerStatus = NodeStatus.success; // решение принято
    }
    nodes.add(AgentNodeData(
      id: "router_${d.stepNo}_$i",
      name: "router",
      role: "router",
      status: routerStatus,
      subtasks: const [],
      artifacts: const [],
      stepNo: d.stepNo,
      kind: NodeKind.router,
      reason: d.reason,
    ));

    for (int aIdx = 0; aIdx < d.chosenAgents.length; aIdx++) {
      final baseAgentName = d.chosenAgents[aIdx];
      int duplicateCount = 0;
      for (int i = 0; i < aIdx; i++) {
        if (d.chosenAgents[i] == baseAgentName) {
          duplicateCount++;
        }
      }
      final agentName = duplicateCount > 0 ? '$baseAgentName ${duplicateCount + 1}' : baseAgentName;

      final teamAgent = teamAgents.firstWhere(
        (a) => a.name == baseAgentName,
        orElse: () => AgentModel(
          id: baseAgentName,
          name: baseAgentName,
          role: assignedAgentName == baseAgentName ? (assignedAgentRole ?? baseAgentName) : baseAgentName,
          isActive: true,
        ),
      );
      final role = teamAgent.role;

      final stepEvents = events.where((e) => 
        e.kind == 'agent_job' && 
        e.agentName == baseAgentName && 
        (e.createdAt.isAfter(start) || e.createdAt.isAtSameMomentAs(start)) && 
        e.createdAt.isBefore(end)
      ).toList();
      TaskEventModel? event;
      if (duplicateCount < stepEvents.length) {
        event = stepEvents[duplicateCount];
      }
      final instructions = event?.instructions;
      final targetArtifactIds = event?.targetArtifactIds;

      NodeStatus status;
      if (isStepRunning && isLast) {
        status = NodeStatus.running;
      } else if (d.outcome == 'failed' || d.outcome == 'needs_human') {
        status = NodeStatus.failed;
      } else if (d.done) {
        status = NodeStatus.success;
      } else {
        status = NodeStatus.pending;
      }

      final nodeArtifacts = <Artifact>[];
      final nodeSubtasks = <String>[];

      for (final art in artifacts) {
        if (art.producerAgent.toLowerCase() == baseAgentName.toLowerCase()) {
          if (targetArtifactIds != null && targetArtifactIds.isNotEmpty) {
            if (art.parentId != null && targetArtifactIds.contains(art.parentId)) {
              nodeArtifacts.add(art);
              if (art.kind == 'subtask_description') {
                final title = art.subtaskTitle ?? art.summary;
                if (title.isNotEmpty && !nodeSubtasks.contains(title)) {
                  nodeSubtasks.add(title);
                }
              }
            }
          } else {
            final isAfterOrEqual = art.createdAt.isAfter(start) || art.createdAt.isAtSameMomentAs(start);
            final isBefore = art.createdAt.isBefore(end);
            if (isAfterOrEqual && isBefore) {
              nodeArtifacts.add(art);
              if (art.kind == 'subtask_description') {
                final title = art.subtaskTitle ?? art.summary;
                if (title.isNotEmpty && !nodeSubtasks.contains(title)) {
                  nodeSubtasks.add(title);
                }
              }
            }
          }
        }
      }

      nodes.add(AgentNodeData(
        id: "${d.stepNo}_${agentName}_$aIdx",
        name: agentName,
        role: role,
        status: status,
        subtasks: nodeSubtasks,
        artifacts: nodeArtifacts,
        stepNo: d.stepNo,
        instructions: instructions,
        targetArtifactIds: targetArtifactIds,
      ));
    }
  }

  if (taskState == 'active' && assignedAgentName != null) {
    final latestDecision = sortedDecisions.last;
    final containsAssigned = latestDecision.chosenAgents.contains(assignedAgentName);
    if (!containsAssigned && latestDecision.done) {
      final nextStepNo = latestDecision.stepNo + 1;
      final role = teamAgents.firstWhere(
        (a) => a.name == assignedAgentName,
        orElse: () => AgentModel(id: assignedAgentName, name: assignedAgentName, role: assignedAgentRole ?? assignedAgentName, isActive: true),
      ).role;
      nodes.add(AgentNodeData(
        id: "${nextStepNo}_${assignedAgentName}_0",
        name: assignedAgentName,
        role: role,
        status: NodeStatus.running,
        subtasks: const [],
        artifacts: const [],
        stepNo: nextStepNo,
      ));
    }
  }

  // Orchestrator root — единая точка входа цикла оркестрации; ветвится в Router первого шага.
  nodes.add(AgentNodeData(
    id: "orchestrator",
    name: "orchestrator",
    role: "orchestrator",
    status: NodeStatus.success,
    subtasks: const [],
    artifacts: const [],
    stepNo: sortedDecisions.first.stepNo,
    kind: NodeKind.orchestrator,
  ));

  return nodes;
}

class TaskExecutionGraph extends ConsumerStatefulWidget {
  const TaskExecutionGraph({
    super.key,
    required this.projectId,
    required this.taskId,
    required this.taskState,
    required this.onAgentSelected,
    this.selectedAgentName,
    this.selectedAgentNodeId,
    this.assignedAgentName,
    this.assignedAgentRole,
  });

  final String projectId;
  final String taskId;
  final String taskState;
  final void Function(AgentNodeData node) onAgentSelected;
  final String? selectedAgentName;
  final String? selectedAgentNodeId;
  final String? assignedAgentName;
  final String? assignedAgentRole;

  @override
  ConsumerState<TaskExecutionGraph> createState() => _TaskExecutionGraphState();
}

class _TaskExecutionGraphState extends ConsumerState<TaskExecutionGraph>
    with SingleTickerProviderStateMixin {
  late final AnimationController _animationController;
  late final TransformationController _transformationController;
  bool _showTimelineOnMobile = true;
  bool _initializedTransformation = false;

  @override
  void initState() {
    super.initState();
    _animationController = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 2),
    );
    final isWidgetTest = WidgetsBinding.instance.runtimeType.toString().contains('Test');
    if (!isWidgetTest) {
      _animationController.repeat(reverse: true);
    }
    _transformationController = TransformationController();
  }

  @override
  void didUpdateWidget(covariant TaskExecutionGraph oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.taskId != widget.taskId) {
      _initializedTransformation = false;
    }
  }

  @override
  void dispose() {
    _animationController.dispose();
    _transformationController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'task_execution_graph');
    final teamAsync = ref.watch(teamProvider(widget.projectId));
    final decisionsAsync = ref.watch(taskRouterDecisionsProvider(widget.taskId));
    final artifactsAsync = ref.watch(taskArtifactsProvider(widget.taskId));
    final eventsAsync = ref.watch(taskEventsProvider(widget.taskId));

    final width = MediaQuery.sizeOf(context).width;
    final isMobile = width < 720;

    return teamAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Center(
        child: Text(
          '${l10n.dataLoadError}: $err',
          style: const TextStyle(color: Colors.red),
        ),
      ),
      data: (team) {
        return decisionsAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => Center(
            child: Text(
              '${l10n.dataLoadError}: $err',
              style: const TextStyle(color: Colors.red),
            ),
          ),
          data: (decisions) {
            return artifactsAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (err, _) => Center(
                child: Text(
                  '${l10n.dataLoadError}: $err',
                  style: const TextStyle(color: Colors.red),
                ),
              ),
              data: (artifacts) {
                return eventsAsync.when(
                  loading: () => const Center(child: CircularProgressIndicator()),
                  error: (err, _) => Center(
                    child: Text(
                      '${l10n.dataLoadError}: $err',
                      style: const TextStyle(color: Colors.red),
                    ),
                  ),
                  data: (events) {
                    // 1. Построение списка нод последовательных шагов
                    final nodesList = buildAgentNodes(
                      decisions: decisions,
                      artifacts: artifacts,
                      events: events,
                      taskState: widget.taskState,
                      assignedAgentName: widget.assignedAgentName,
                      assignedAgentRole: widget.assignedAgentRole,
                      teamAgents: team.agents,
                    );

                // 2. Группировка по «уровню» и построение уровней (самый свежий наверху).
                //    Уровень кодирует порядок Orchestrator → Router(step) → агенты(step) →
                //    Router(step+1) → ...: orchestrator = minStep*2-1, router = step*2,
                //    агенты = step*2+1. Так существующая логика «рёбра между соседними
                //    уровнями» автоматически даёт нужную цепочку, и Router/Orchestrator
                //    встают отдельными рядами между шагами.
                final int minStep = decisions.isEmpty
                    ? 0
                    : decisions.map((d) => d.stepNo).reduce(min);
                int levelOf(AgentNodeData n) {
                  switch (n.kind) {
                    case NodeKind.orchestrator:
                      return minStep * 2 - 1;
                    case NodeKind.router:
                      return n.stepNo * 2;
                    case NodeKind.agent:
                      return n.stepNo * 2 + 1;
                  }
                }

                final stepGroups = <int, List<AgentNodeData>>{};
                for (final node in nodesList) {
                  stepGroups.putIfAbsent(levelOf(node), () => []).add(node);
                }

                final sortedStepNos = stepGroups.keys.toList()..sort((a, b) => b.compareTo(a));

                final levels = sortedStepNos.map((step) {
                  return stepGroups[step]!.map((node) => node.id).toList();
                }).toList();

                // 3. Рёбра между соседними уровнями (полное двудольное соединение).
                final sortedStepNosAsc = stepGroups.keys.toList()..sort();
                final edges = <(String, String)>[];
                for (int i = 0; i < sortedStepNosAsc.length - 1; i++) {
                  final fromStep = sortedStepNosAsc[i];
                  final toStep = sortedStepNosAsc[i + 1];
                  final fromNodes = stepGroups[fromStep]!;
                  final toNodes = stepGroups[toStep]!;
                  for (final fromNode in fromNodes) {
                    for (final toNode in toNodes) {
                      edges.add((fromNode.id, toNode.id));
                    }
                  }
                }

                // Активный агент/нода для отрисовки подсветки связей (первый из запущенных)
                AgentNodeData? activeNode;
                try {
                  activeNode = nodesList.firstWhere((n) => n.status == NodeStatus.running);
                } catch (_) {
                  activeNode = null;
                }
                final String? activeAgent = activeNode?.id;

                if (isMobile) {
                  return DefaultTabController(
                    length: 2,
                    initialIndex: _showTimelineOnMobile ? 0 : 1,
                    child: Column(
                      children: [
                        TabBar(
                          tabs: [
                            Tab(text: l10n.agentMatrixTimelineTab),
                            Tab(text: l10n.agentMatrixGraphTab),
                          ],
                          onTap: (index) {
                            setState(() {
                              _showTimelineOnMobile = index == 0;
                            });
                          },
                        ),
                        Expanded(
                          child: TabBarView(
                            physics: const NeverScrollableScrollPhysics(),
                            children: [
                              _buildMobileTimeline(context, decisions, nodesList),
                              _build2DCanvas(context, nodesList, edges, activeAgent, levels),
                            ],
                          ),
                        ),
                      ],
                    ),
                  );
                } else {
                  return _build2DCanvas(context, nodesList, edges, activeAgent, levels);
                }
                  },
                );
              },
            );
          },
        );
      },
    );
  }

  Widget _buildMobileTimeline(
    BuildContext context,
    List<RouterDecision> decisions,
    List<AgentNodeData> nodes,
  ) {
    return ListView.builder(
      padding: const EdgeInsets.all(16),
      itemCount: decisions.length,
      itemBuilder: (context, index) {
        final d = decisions[index];
        final time = DateFormat.Hm().format(d.createdAt.toLocal());
        final isRunning = !d.done && widget.taskState == 'active';
        final isFailed = d.outcome == 'failed' || d.outcome == 'needs_human';

        Color indicatorColor = Colors.grey;
        IconData icon = Icons.hourglass_empty;

        if (isRunning) {
          indicatorColor = Colors.blue;
          icon = Icons.sync;
        } else if (isFailed) {
          indicatorColor = Colors.red;
          icon = Icons.error_outline;
        } else if (d.done) {
          indicatorColor = Colors.grey.shade500;
          icon = Icons.check_circle_outline;
        }

        return IntrinsicHeight(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Column(
                children: [
                  Container(
                    width: 24,
                    height: 24,
                    decoration: BoxDecoration(
                      color: indicatorColor.withValues(alpha: 0.15),
                      shape: BoxShape.circle,
                      border: Border.all(color: indicatorColor, width: 2),
                    ),
                    child: Icon(icon, size: 14, color: indicatorColor),
                  ),
                  if (index < decisions.length - 1)
                    Expanded(
                      child: Container(
                        width: 2,
                        color: Colors.grey.shade300,
                      ),
                    ),
                ],
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Padding(
                  padding: const EdgeInsets.only(bottom: 16),
                  child: Card(
                    margin: EdgeInsets.zero,
                    child: InkWell(
                      onTap: () {
                        if (d.chosenAgents.isNotEmpty) {
                          final name = d.chosenAgents.first;
                          final node = nodes.firstWhere(
                            (n) => n.stepNo == d.stepNo && n.name == name,
                            orElse: () => nodes.firstWhere((n) => n.name == name),
                          );
                          widget.onAgentSelected(node);
                        }
                      },
                      child: Padding(
                        padding: const EdgeInsets.all(12),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(
                              children: [
                                Text(
                                  'Шаг ${d.stepNo}',
                                  style: const TextStyle(fontWeight: FontWeight.bold),
                                ),
                                const Spacer(),
                                Text(time, style: Theme.of(context).textTheme.bodySmall),
                              ],
                            ),
                            const SizedBox(height: 6),
                            Text(
                              d.chosenAgents.join(' · '),
                              style: const TextStyle(
                                fontFamily: 'monospace',
                                fontWeight: FontWeight.w600,
                                color: Colors.blueGrey,
                              ),
                            ),
                            const SizedBox(height: 4),
                            Text(
                              d.reason,
                              style: Theme.of(context).textTheme.bodySmall,
                            ),
                            if (d.outcome != null) ...[
                              const SizedBox(height: 6),
                              Container(
                                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                                decoration: BoxDecoration(
                                  color: indicatorColor.withValues(alpha: 0.1),
                                  borderRadius: BorderRadius.circular(4),
                                ),
                                child: Text(
                                  d.outcome!.toUpperCase(),
                                  style: TextStyle(
                                    fontSize: 10,
                                    fontWeight: FontWeight.bold,
                                    color: indicatorColor,
                                  ),
                                ),
                              ),
                            ],
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _build2DCanvas(
    BuildContext context,
    List<AgentNodeData> nodes,
    List<(String, String)> edges,
    String? activeAgent,
    List<List<String>> levels,
  ) {
    const double cardWidth = 180.0;
    const double cardHeight = 90.0;
    const double horizontalSpacing = 40.0;
    const double verticalSpacing = 140.0;

    return LayoutBuilder(
      builder: (context, constraints) {
        final double canvasWidth = max(constraints.maxWidth, 800.0);
        final double canvasHeight = max(constraints.maxHeight, levels.length * verticalSpacing + 100.0);

        final nodePositions = <String, Offset>{};
        for (int lvlIdx = 0; lvlIdx < levels.length; lvlIdx++) {
          final lvlAgents = levels[lvlIdx];
          final numInLvl = lvlAgents.length;
          final double totalWidth = numInLvl * cardWidth + (numInLvl - 1) * horizontalSpacing;
          final double y = lvlIdx * verticalSpacing + 50.0;

          for (int i = 0; i < numInLvl; i++) {
            final nodeId = lvlAgents[i];
            final double x = (canvasWidth / 2) - (totalWidth / 2) + i * (cardWidth + horizontalSpacing);
            nodePositions[nodeId] = Offset(x, y);
          }
        }

        if (!_initializedTransformation && nodePositions.isNotEmpty) {
          double minX = double.infinity;
          double maxX = -double.infinity;
          double minY = double.infinity;
          double maxY = -double.infinity;

          for (final pos in nodePositions.values) {
            minX = min(minX, pos.dx);
            maxX = max(maxX, pos.dx + cardWidth);
            minY = min(minY, pos.dy);
            maxY = max(maxY, pos.dy + cardHeight);
          }

          final graphWidth = (maxX - minX) + 160.0;
          final graphHeight = (maxY - minY) + 100.0;

          final double scaleX = constraints.maxWidth / graphWidth;
          final double scaleY = constraints.maxHeight / graphHeight;
          final double fitScale = min(scaleX, scaleY).clamp(0.2, 1.0);

          final double dx = (constraints.maxWidth - graphWidth * fitScale) / 2;
          final double dy = (constraints.maxHeight - graphHeight * fitScale) / 2;

          _transformationController.value = Matrix4.identity()
            ..translate(dx, dy)
            ..scale(fitScale);
          _initializedTransformation = true;
        }

        return InteractiveViewer(
          transformationController: _transformationController,
          constrained: false,
          minScale: 0.2,
          maxScale: 2.0,
          child: AnimatedBuilder(
            animation: _animationController,
            builder: (context, child) {
              final canvasSize = Size(canvasWidth, canvasHeight);
              return CustomPaint(
                size: canvasSize,
                painter: GraphConnectionsPainter(
                  nodePositions: nodePositions,
                  edges: edges,
                  activeAgent: activeAgent,
                  animationValue: _animationController.value,
                ),
                child: SizedBox(
                  width: canvasSize.width,
                  height: canvasSize.height,
                  child: Stack(
                    children: nodes.map((node) {
                      final pos = nodePositions[node.id] ?? Offset.zero;
                      return Positioned(
                        left: pos.dx,
                        top: pos.dy,
                        child: _buildAgentCard(node),
                      );
                    }).toList(),
                  ),
                ),
              );
            },
          ),
        );
      },
    );
  }

  Widget _buildAgentCard(AgentNodeData node) {
    final l10n = requireAppLocalizations(context, where: 'task_execution_graph');
    final isSelected = widget.selectedAgentNodeId == node.id || (widget.selectedAgentNodeId == null && widget.selectedAgentName == node.name);

    Color color;
    IconData icon;
    String statusText;

    switch (node.status) {
      case NodeStatus.pending:
        color = Colors.grey;
        icon = Icons.hourglass_empty;
        statusText = l10n.agentMatrixStatusPending;
      case NodeStatus.running:
        color = Colors.blue;
        icon = Icons.sync;
        statusText = l10n.agentMatrixStatusRunning;
      case NodeStatus.success:
        // Завершённые ноды — нейтрально-серые (без «зелёного успеха»).
        color = Colors.grey.shade500;
        icon = Icons.check_circle_outline;
        statusText = l10n.agentMatrixStatusSuccess;
      case NodeStatus.failed:
        color = Colors.red;
        icon = Icons.error_outline;
        statusText = l10n.agentMatrixStatusFailed;
    }

    // Router/Orchestrator — отдельные иконки, чтобы визуально отличать «мозг» от исполнителей.
    if (node.kind == NodeKind.router) {
      icon = Icons.call_split;
    } else if (node.kind == NodeKind.orchestrator) {
      icon = Icons.hub_outlined;
    }
    final isControlNode = node.kind != NodeKind.agent;

    final isRunning = node.status == NodeStatus.running;
    final pulseValue = isRunning
        ? 1.0 + 0.05 * sin(_animationController.value * 2 * pi)
        : 1.0;

    return Transform.scale(
      scale: pulseValue,
      child: Container(
        width: 180,
        height: 90,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isSelected ? Colors.deepPurple : color.withValues(alpha: 0.7),
            width: isSelected ? 3 : (isRunning ? 2.5 : 1.5),
          ),
          boxShadow: [
            if (isRunning)
              BoxShadow(
                color: color.withValues(alpha: 0.3),
                blurRadius: 12,
                spreadRadius: 2,
              ),
            BoxShadow(
              color: Colors.black.withValues(alpha: 0.05),
              blurRadius: 6,
              offset: const Offset(0, 3),
            ),
          ],
        ),
        child: Material(
          borderRadius: BorderRadius.circular(11),
          color: Colors.white.withValues(alpha: 0.9),
          clipBehavior: Clip.antiAlias,
          child: InkWell(
            // Кликабельны только агенты (инспектор показывает агента). Router/Orchestrator
            // несут reason прямо на карточке — открывать по ним нечего.
            onTap: isControlNode ? null : () => widget.onAgentSelected(node),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Container(
                        padding: const EdgeInsets.all(4),
                        decoration: BoxDecoration(
                          color: color.withValues(alpha: 0.1),
                          shape: BoxShape.circle,
                        ),
                        child: Icon(icon, size: 14, color: color),
                      ),
                      const SizedBox(width: 6),
                      Expanded(
                        child: Text(
                          node.name,
                          style: const TextStyle(
                            fontWeight: FontWeight.bold,
                            fontSize: 13,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
                  const Spacer(),
                  Text(
                    node.role.toUpperCase(),
                    style: TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.w600,
                      color: Colors.grey.shade600,
                    ),
                  ),
                  // Для router-ноды показываем краткий reason решения (полный — в timeline).
                  if (node.reason != null && node.reason!.isNotEmpty) ...[
                    const SizedBox(height: 2),
                    Text(
                      node.reason!,
                      style: TextStyle(fontSize: 9, color: Colors.grey.shade600),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                  const SizedBox(height: 2),
                  Row(
                    children: [
                      Text(
                        statusText,
                        style: TextStyle(
                          fontSize: 10,
                          fontWeight: FontWeight.bold,
                          color: color,
                        ),
                      ),
                      const Spacer(),
                      if (node.subtasks.isNotEmpty)
                        Text(
                          '${node.subtasks.length} sub',
                          style: TextStyle(
                            fontSize: 9,
                            color: Colors.grey.shade500,
                          ),
                        ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class GraphConnectionsPainter extends CustomPainter {
  final Map<String, Offset> nodePositions;
  final List<(String, String)> edges;
  final String? activeAgent;
  final double animationValue;

  GraphConnectionsPainter({
    required this.nodePositions,
    required this.edges,
    required this.activeAgent,
    required this.animationValue,
  });

  @override
  void paint(Canvas canvas, Size size) {
    final linePaint = Paint()
      ..color = Colors.grey.shade300
      ..strokeWidth = 2.0
      ..style = PaintingStyle.stroke;

    final activeLinePaint = Paint()
      ..color = Colors.blue.shade400
      ..strokeWidth = 3.0
      ..style = PaintingStyle.stroke;

    // Отрисовка всех ребер
    for (final edge in edges) {
      final p1 = nodePositions[edge.$1];
      final p2 = nodePositions[edge.$2];

      if (p1 != null && p2 != null) {
        final Offset start;
        final Offset end;
        final bool isVertical = (p1.dy - p2.dy).abs() > 10.0;

        if (isVertical) {
          if (p1.dy > p2.dy) {
            // Рисуем снизу вверх (указывает вверх)
            start = Offset(p1.dx + 90, p1.dy);
            end = Offset(p2.dx + 90, p2.dy + 90);
          } else {
            // Рисуем сверху вниз (указывает вниз)
            start = Offset(p1.dx + 90, p1.dy + 90);
            end = Offset(p2.dx + 90, p2.dy);
          }
        } else {
          // Горизонтально
          if (p1.dx < p2.dx) {
            start = Offset(p1.dx + 180, p1.dy + 45);
            end = Offset(p2.dx, p2.dy + 45);
          } else {
            start = Offset(p1.dx, p1.dy + 45);
            end = Offset(p2.dx + 180, p2.dy + 45);
          }
        }

        final isActive = edge.$2 == activeAgent;

        if (isActive) {
          _drawActiveConnection(canvas, start, end, activeLinePaint, isVertical: isVertical, pointingUp: p1.dy > p2.dy, pointingRight: p2.dx > p1.dx);
        } else {
          _drawStandardConnection(canvas, start, end, linePaint, isVertical: isVertical, pointingUp: p1.dy > p2.dy, pointingRight: p2.dx > p1.dx);
        }
      }
    }
  }

  void _drawStandardConnection(Canvas canvas, Offset p1, Offset p2, Paint paint, {required bool isVertical, required bool pointingUp, required bool pointingRight}) {
    final path = Path();
    path.moveTo(p1.dx, p1.dy);

    final Offset controlPoint1;
    final Offset controlPoint2;

    if (isVertical) {
      final double cpOffset = pointingUp ? -40.0 : 40.0;
      controlPoint1 = Offset(p1.dx, p1.dy + cpOffset);
      controlPoint2 = Offset(p2.dx, p2.dy - cpOffset);
    } else {
      final double cpOffset = pointingRight ? 60.0 : -60.0;
      controlPoint1 = Offset(p1.dx + cpOffset, p1.dy);
      controlPoint2 = Offset(p2.dx - cpOffset, p2.dy);
    }

    path.cubicTo(
      controlPoint1.dx,
      controlPoint1.dy,
      controlPoint2.dx,
      controlPoint2.dy,
      p2.dx,
      p2.dy,
    );

    canvas.drawPath(path, paint);
    _drawArrowHead(canvas, p2, paint.color, isVertical: isVertical, pointingUp: pointingUp, pointingRight: pointingRight);
  }

  void _drawActiveConnection(Canvas canvas, Offset p1, Offset p2, Paint paint, {required bool isVertical, required bool pointingUp, required bool pointingRight}) {
    final path = Path();
    path.moveTo(p1.dx, p1.dy);

    final Offset controlPoint1;
    final Offset controlPoint2;

    if (isVertical) {
      final double cpOffset = pointingUp ? -40.0 : 40.0;
      controlPoint1 = Offset(p1.dx, p1.dy + cpOffset);
      controlPoint2 = Offset(p2.dx, p2.dy - cpOffset);
    } else {
      final double cpOffset = pointingRight ? 60.0 : -60.0;
      controlPoint1 = Offset(p1.dx + cpOffset, p1.dy);
      controlPoint2 = Offset(p2.dx - cpOffset, p2.dy);
    }

    path.cubicTo(
      controlPoint1.dx,
      controlPoint1.dy,
      controlPoint2.dx,
      controlPoint2.dy,
      p2.dx,
      p2.dy,
    );

    // Отрисовка сплошного контура
    canvas.drawPath(path, paint);

    // Анимация бегущих точек по пути
    final pathMetrics = path.computeMetrics();
    for (final metric in pathMetrics) {
      final length = metric.length;
      final step = 25.0; // Шаг между точками
      final startOffset = (animationValue * length) % step;

      for (double d = startOffset; d < length; d += step) {
        final tangent = metric.getTangentForOffset(d);
        if (tangent != null) {
          canvas.drawCircle(
            tangent.position,
            3.5,
            Paint()..color = Colors.blue.shade800,
          );
        }
      }
    }

    _drawArrowHead(canvas, p2, paint.color, isVertical: isVertical, pointingUp: pointingUp, pointingRight: pointingRight);
  }

  void _drawArrowHead(Canvas canvas, Offset tip, Color color, {required bool isVertical, required bool pointingUp, required bool pointingRight}) {
    final arrowPaint = Paint()
      ..color = color
      ..style = PaintingStyle.fill;

    final path = Path();
    if (isVertical) {
      final double dyOffset = pointingUp ? 10.0 : -10.0;
      path.moveTo(tip.dx, tip.dy);
      path.lineTo(tip.dx - 6, tip.dy + dyOffset);
      path.lineTo(tip.dx + 6, tip.dy + dyOffset);
      path.close();
    } else {
      final double dxOffset = pointingRight ? -10.0 : 10.0;
      path.moveTo(tip.dx, tip.dy);
      path.lineTo(tip.dx + dxOffset, tip.dy - 6);
      path.lineTo(tip.dx + dxOffset, tip.dy + 6);
      path.close();
    }

    canvas.drawPath(path, arrowPaint);
  }

  @override
  bool shouldRepaint(covariant GraphConnectionsPainter oldDelegate) {
    return oldDelegate.animationValue != animationValue ||
        oldDelegate.activeAgent != activeAgent ||
        oldDelegate.edges.length != edges.length;
  }
}
