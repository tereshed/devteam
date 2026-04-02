import 'package:dio/dio.dart';
import 'package:frontend/features/admin/workflows/domain/execution_model.dart';
import 'package:frontend/features/admin/workflows/domain/workflow_model.dart';
import 'package:retrofit/retrofit.dart';

part 'workflows_repository.g.dart';

@RestApi()
abstract class WorkflowsRepository {
  factory WorkflowsRepository(Dio dio, {String baseUrl}) = _WorkflowsRepository;

  @GET('/workflows')
  Future<List<Workflow>> getWorkflows();

  @POST('/workflows/{name}/start')
  Future<Execution> startWorkflow(
    @Path('name') String name,
    @Body() Map<String, dynamic> body,
  );

  @GET('/executions')
  Future<ExecutionListResponse> getExecutions(
    @Query('limit') int? limit,
    @Query('offset') int? offset,
  );

  @GET('/executions/{id}')
  Future<Execution> getExecution(@Path('id') String id);

  @GET('/executions/{id}/steps')
  Future<List<ExecutionStep>> getExecutionSteps(@Path('id') String id);
}
