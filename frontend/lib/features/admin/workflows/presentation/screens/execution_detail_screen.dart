import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/admin/workflows/data/workflows_providers.dart';

class ExecutionDetailScreen extends ConsumerWidget {
  final String id;

  const ExecutionDetailScreen({super.key, required this.id});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final executionAsync = ref.watch(executionDetailProvider(id));
    final stepsAsync = ref.watch(executionStepsProvider(id));

    return Scaffold(
      appBar: AppBar(title: Text('Execution ${id.substring(0, 8)}')),
      body: executionAsync.when(
        data: (exec) => SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildHeader(context, exec),
              const SizedBox(height: 16),
              _buildDataSection(context, 'Input Data', exec.inputData),
              const SizedBox(height: 16),
              _buildDataSection(context, 'Output Data', exec.outputData),
              const SizedBox(height: 24),
              Text('Timeline', style: Theme.of(context).textTheme.titleLarge),
              const SizedBox(height: 8),
              stepsAsync.when(
                data: (steps) => _buildTimeline(context, steps),
                loading: () => const Center(child: LinearProgressIndicator()),
                error: (err, _) => Text('Error loading steps: $err'),
              ),
            ],
          ),
        ),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(child: Text('Error: $err')),
      ),
    );
  }

  Widget _buildHeader(BuildContext context, dynamic exec) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  'Status: ${exec.status.toUpperCase()}',
                  style: const TextStyle(fontWeight: FontWeight.bold),
                ),
                Text('Steps: ${exec.stepCount}'),
              ],
            ),
            const SizedBox(height: 8),
            if (exec.errorMessage != null)
              Text(
                'Error: ${exec.errorMessage}',
                style: const TextStyle(color: Colors.red),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildDataSection(BuildContext context, String title, String? data) {
    if (data == null || data.isEmpty) return const SizedBox.shrink();
    return ExpansionTile(
      title: Text(title),
      children: [
        Container(
          width: double.infinity,
          padding: const EdgeInsets.all(16),
          color: Colors.grey[100],
          child: SelectableText(
            data,
            style: const TextStyle(fontFamily: 'monospace'),
          ),
        ),
      ],
    );
  }

  Widget _buildTimeline(BuildContext context, List<dynamic> steps) {
    return ListView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      itemCount: steps.length,
      itemBuilder: (context, index) {
        final step = steps[index];
        return Card(
          margin: const EdgeInsets.symmetric(vertical: 4),
          child: ExpansionTile(
            leading: CircleAvatar(child: Text('${index + 1}')),
            title: Text(step.agentName ?? step.stepId),
            subtitle: Text('${step.tokensUsed} tokens • ${step.durationMs}ms'),
            children: [
              Padding(
                padding: const EdgeInsets.all(16.0),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      'Input Context:',
                      style: TextStyle(fontWeight: FontWeight.bold),
                    ),
                    Text(step.inputContext ?? ''),
                    const Divider(),
                    const Text(
                      'Output Content:',
                      style: TextStyle(fontWeight: FontWeight.bold),
                    ),
                    Text(step.outputContent ?? ''),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}
