import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/admin/workflows/data/workflows_providers.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

class ExecutionsListScreen extends ConsumerWidget {
  const ExecutionsListScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final executionsAsync = ref.watch(executionsListProvider());

    return Scaffold(
      appBar: AppBar(title: const Text('Executions History')),
      body: executionsAsync.when(
        data: (response) => ListView.builder(
          itemCount: response.executions.length,
          itemBuilder: (context, index) {
            final exec = response.executions[index];
            return Card(
              margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
              child: ListTile(
                onTap: () => context.push('/admin/executions/${exec.id}'),
                leading: _buildStatusIcon(exec.status),
                title: Text(
                  exec.id.substring(0, 8),
                  style: const TextStyle(fontFamily: 'monospace'),
                ),
                subtitle: Text(
                  'Steps: ${exec.stepCount} • ${DateFormat.yMMMd().add_jm().format(exec.createdAt)}',
                ),
                trailing: const Icon(Icons.chevron_right),
              ),
            );
          },
        ),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, stack) => Center(child: Text('Error: $err')),
      ),
    );
  }

  Widget _buildStatusIcon(String status) {
    Color color;
    IconData icon;
    switch (status) {
      case 'completed':
        color = Colors.green;
        icon = Icons.check_circle;
        break;
      case 'failed':
        color = Colors.red;
        icon = Icons.error;
        break;
      case 'running':
        color = Colors.blue;
        icon = Icons.autorenew;
        break;
      default:
        color = Colors.grey;
        icon = Icons.help;
    }
    return Icon(icon, color: color);
  }
}
