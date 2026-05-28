import 'dart:math';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:intl/intl.dart';

enum NodeStatus { pending, running, success, failed }

class AgentNodeData {
  final String name;
  final String role;
  final NodeStatus status;
  final List<String> subtasks;
  final List<Artifact> artifacts;

  const AgentNodeData({
    required this.name,
    required this.role,
    required this.status,
    required this.subtasks,
    required this.artifacts,
  });
}

class TaskExecutionGraph extends ConsumerStatefulWidget {
  const TaskExecutionGraph({
    super.key,
    required this.projectId,
    required this.taskId,
    required this.taskState,
    required this.onAgentSelected,
    this.selectedAgentName,
  });

  final String projectId;
  final String taskId;
  final String taskState;
  final void Function(AgentNodeData node) onAgentSelected;
  final String? selectedAgentName;

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
                // 1. Сбор всех уникальных агентов
                final teamAgents = team.agents;
                final allAgentNames = <String>{};
                for (final a in teamAgents) {
                  allAgentNames.add(a.name);
                }
                for (final d in decisions) {
                  allAgentNames.addAll(d.chosenAgents);
                }

                // 2. Определение статусов агентов
                final agentStatuses = <String, NodeStatus>{};
                for (final name in allAgentNames) {
                  agentStatuses[name] = NodeStatus.pending;
                }

                // Пройдемся по решениям для вычисления статуса
                String? activeAgent;
                for (final d in decisions) {
                  final isLast = d == decisions.last;
                  final isStepRunning = !d.done && widget.taskState == 'active';

                  for (final agentName in d.chosenAgents) {
                    if (isStepRunning && isLast) {
                      agentStatuses[agentName] = NodeStatus.running;
                      activeAgent = agentName;
                    } else {
                      final outcome = d.outcome;
                      if (outcome == 'failed' || outcome == 'needs_human') {
                        agentStatuses[agentName] = NodeStatus.failed;
                      } else {
                        // Если уже был failed, не перетираем на success
                        if (agentStatuses[agentName] != NodeStatus.failed) {
                          agentStatuses[agentName] = NodeStatus.success;
                        }
                      }
                    }
                  }
                }

                // 3. Группировка подзадач и артефактов по агентам
                final agentSubtasks = <String, List<String>>{};
                final agentArtifacts = <String, List<Artifact>>{};

                for (final name in allAgentNames) {
                  agentSubtasks[name] = [];
                  agentArtifacts[name] = [];
                }

                // Подзадачи из артефактов
                for (final art in artifacts) {
                  if (art.kind == 'subtask_description') {
                    final agent = art.producerAgent;
                    if (allAgentNames.contains(agent)) {
                      final title = art.subtaskTitle ?? art.summary;
                      if (title.isNotEmpty && !agentSubtasks[agent]!.contains(title)) {
                        agentSubtasks[agent]!.add(title);
                      }
                    }
                  }
                  final agent = art.producerAgent;
                  if (allAgentNames.contains(agent)) {
                    agentArtifacts[agent]!.add(art);
                  }
                }

                // 4. Построение графа
                final nodesList = allAgentNames.map((name) {
                  final teamAgent = teamAgents.firstWhere(
                    (a) => a.name == name,
                    orElse: () => AgentModel(
                      id: name,
                      name: name,
                      role: name,
                      isActive: true,
                    ),
                  );
                  return AgentNodeData(
                    name: name,
                    role: teamAgent.role,
                    status: agentStatuses[name] ?? NodeStatus.pending,
                    subtasks: agentSubtasks[name] ?? [],
                    artifacts: agentArtifacts[name] ?? [],
                  );
                }).toList();

                // Выделение переходов (Edges)
                final edges = <(String, String)>[];
                for (int i = 0; i < decisions.length - 1; i++) {
                  final fromAgents = decisions[i].chosenAgents;
                  final toAgents = decisions[i + 1].chosenAgents;
                  for (final from in fromAgents) {
                    for (final to in toAgents) {
                      if (from != to && !edges.contains((from, to))) {
                        edges.add((from, to));
                      }
                    }
                  }
                }

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
                              _build2DCanvas(context, nodesList, edges, activeAgent),
                            ],
                          ),
                        ),
                      ],
                    ),
                  );
                } else {
                  return _build2DCanvas(context, nodesList, edges, activeAgent);
                }
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
    final l10n = requireAppLocalizations(context, where: 'task_execution_graph');
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
          indicatorColor = Colors.green;
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
                          final node = nodes.firstWhere((n) => n.name == name);
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
  ) {
    // Детерминированная раскладка нод по колонкам (ролям)
    final nodePositions = <String, Offset>{};
    final stageColumns = <String, int>{
      'planner': 0,
      'supervisor': 0,
      'decomposer': 1,
      'developer': 2,
      'worker': 2,
      'reviewer': 3,
      'tester': 4,
      'devops': 4,
      'merger': 5,
    };

    final columnCounts = <int, int>{};
    for (final node in nodes) {
      final role = node.role.toLowerCase();
      final col = stageColumns[role] ?? 5;
      columnCounts[col] = (columnCounts[col] ?? 0) + 1;
    }

    final columnCurrentIndex = <int, int>{};
    for (final node in nodes) {
      final role = node.role.toLowerCase();
      final col = stageColumns[role] ?? 5;
      final totalInCol = columnCounts[col] ?? 1;
      final currentIdx = columnCurrentIndex[col] ?? 0;
      columnCurrentIndex[col] = currentIdx + 1;

      // Координаты X и Y
      final double x = col * 260.0 + 80.0;
      // Центрируем ноды вертикально в колонке
      final double totalHeight = totalInCol * 130.0;
      final double y = 250.0 - (totalHeight / 2) + (currentIdx * 130.0) + 50.0;

      nodePositions[node.name] = Offset(x, y);
    }

    return LayoutBuilder(
      builder: (context, constraints) {
        if (!_initializedTransformation && nodePositions.isNotEmpty) {
          double minX = double.infinity;
          double maxX = -double.infinity;
          double minY = double.infinity;
          double maxY = -double.infinity;

          for (final pos in nodePositions.values) {
            minX = min(minX, pos.dx);
            maxX = max(maxX, pos.dx + 180.0);
            minY = min(minY, pos.dy);
            maxY = max(maxY, pos.dy + 90.0);
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
              final canvasSize = Size(
                max(constraints.maxWidth, 6 * 260.0 + 200.0),
                max(constraints.maxHeight, 600.0),
              );
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
                      final pos = nodePositions[node.name] ?? Offset.zero;
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
    final isSelected = widget.selectedAgentName == node.name;

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
        color = Colors.green;
        icon = Icons.check_circle_outline;
        statusText = l10n.agentMatrixStatusSuccess;
      case NodeStatus.failed:
        color = Colors.red;
        icon = Icons.error_outline;
        statusText = l10n.agentMatrixStatusFailed;
    }

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
            onTap: () => widget.onAgentSelected(node),
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
        // Координаты краев карточек
        final start = Offset(p1.dx + 180, p1.dy + 45);
        final end = Offset(p2.dx, p2.dy + 45);

        final isActive = edge.$2 == activeAgent;

        if (isActive) {
          // Рисуем активную линию с движущимися стрелками
          _drawActiveConnection(canvas, start, end, activeLinePaint);
        } else {
          // Рисуем обычную линию
          _drawStandardConnection(canvas, start, end, linePaint);
        }
      }
    }
  }

  void _drawStandardConnection(Canvas canvas, Offset p1, Offset p2, Paint paint) {
    final path = Path();
    path.moveTo(p1.dx, p1.dy);

    // Рисуем плавную кривую Безье
    final controlPoint1 = Offset(p1.dx + 60, p1.dy);
    final controlPoint2 = Offset(p2.dx - 60, p2.dy);
    path.cubicTo(
      controlPoint1.dx,
      controlPoint1.dy,
      controlPoint2.dx,
      controlPoint2.dy,
      p2.dx,
      p2.dy,
    );

    canvas.drawPath(path, paint);
    _drawArrowHead(canvas, p2, paint.color);
  }

  void _drawActiveConnection(Canvas canvas, Offset p1, Offset p2, Paint paint) {
    final path = Path();
    path.moveTo(p1.dx, p1.dy);

    final controlPoint1 = Offset(p1.dx + 60, p1.dy);
    final controlPoint2 = Offset(p2.dx - 60, p2.dy);
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

    _drawArrowHead(canvas, p2, paint.color);
  }

  void _drawArrowHead(Canvas canvas, Offset tip, Color color) {
    final arrowPaint = Paint()
      ..color = color
      ..style = PaintingStyle.fill;

    final path = Path()
      ..moveTo(tip.dx, tip.dy)
      ..lineTo(tip.dx - 10, tip.dy - 6)
      ..lineTo(tip.dx - 10, tip.dy + 6)
      ..close();

    canvas.drawPath(path, arrowPaint);
  }

  @override
  bool shouldRepaint(covariant GraphConnectionsPainter oldDelegate) {
    return oldDelegate.animationValue != animationValue ||
        oldDelegate.activeAgent != activeAgent ||
        oldDelegate.edges.length != edges.length;
  }
}
