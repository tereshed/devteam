import 'package:dio/dio.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';

class MyAgentsRepository {
  MyAgentsRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Future<AgentV2Page> list({CancelToken? cancelToken}) async {
    final response = await _dio.get(
      '/me/agents',
      cancelToken: cancelToken,
    );
    final json = response.data as Map<String, dynamic>;
    return AgentV2Page.fromJson(json);
  }
}
