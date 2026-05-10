import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/features/tasks/data/task_repository.dart';
import 'package:mockito/annotations.dart';

@GenerateNiceMocks([
  MockSpec<TaskRepository>(),
  MockSpec<WebSocketService>(),
])
void main() {}
