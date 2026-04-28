import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/admin/workflows/data/workflows_providers.dart';
import 'package:frontend/features/admin/workflows/domain/models.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

class WorkflowsListScreen extends ConsumerWidget {
  const WorkflowsListScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final workflowsAsync = ref.watch(workflowsListProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Workflows'),
        actions: [
          IconButton(
            icon: const Icon(Icons.history),
            onPressed: () => context.push('/admin/executions'),
            tooltip: 'Executions History',
          ),
        ],
      ),
      body: workflowsAsync.when(
        data: (workflows) => ListView.builder(
          itemCount: workflows.length,
          itemBuilder: (context, index) {
            final wf = workflows[index];
            return Card(
              margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              child: ListTile(
                title: Text(
                  wf.name,
                  style: const TextStyle(fontWeight: FontWeight.bold),
                ),
                subtitle: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (wf.description != null) Text(wf.description!),
                    const SizedBox(height: 4),
                    Text(
                      'Created: ${DateFormat.yMMMd().format(wf.createdAt)}',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ],
                ),
                trailing: FilledButton.icon(
                  onPressed: () => _showRunDialog(context, ref, wf),
                  icon: const Icon(Icons.play_arrow),
                  label: const Text('Run'),
                ),
              ),
            );
          },
        ),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, stack) => Center(child: Text('Error: $err')),
      ),
    );
  }

  Future<void> _showRunDialog(
    BuildContext context,
    WidgetRef ref,
    Workflow wf,
  ) async {
    final inputController = TextEditingController();
    return showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Text('Run ${wf.name}'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Text('Enter input data for the workflow:'),
            const SizedBox(height: 16),
            TextField(
              controller: inputController,
              maxLines: 5,
              decoration: const InputDecoration(
                border: OutlineInputBorder(),
                hintText: 'Input text or JSON...',
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () async {
              try {
                Navigator.pop(context); // Close dialog first
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('Starting workflow...')),
                );

                final execution = await ref
                    .read(workflowsRepositoryProvider)
                    .startWorkflow(wf.name, {'input': inputController.text});

                if (context.mounted) {
                  context.push('/admin/executions/${execution.id}');
                }
              } catch (e) {
                if (context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                      content: Text('Error: $e'),
                      backgroundColor: Colors.red,
                    ),
                  );
                }
              }
            },
            child: const Text('Start'),
          ),
        ],
      ),
    );
  }
}
