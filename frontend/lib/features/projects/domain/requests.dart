import 'package:freezed_annotation/freezed_annotation.dart';

part 'requests.freezed.dart';
part 'requests.g.dart';

/// Фильтр для списка проектов
@freezed
abstract class ProjectListFilter with _$ProjectListFilter {
  const factory ProjectListFilter({
    String? status,
    String? gitProvider,
    String? search,
    @Default('created_at') String orderBy,
    @Default('DESC') String orderDir,
  }) = _ProjectListFilter;

  const ProjectListFilter._();

  /// Преобразует фильтр в query parameters для Dio GET запроса
  Map<String, dynamic> toQueryParameters() {
    return {
      if (status != null) 'status': status,
      if (gitProvider != null) 'git_provider': gitProvider,
      if (search != null && search!.isNotEmpty) 'search': search,
      'order_by': orderBy,
      'order_dir': orderDir,
    };
  }
}

/// Ответ на запрос списка проектов
@freezed
abstract class ProjectListResponse with _$ProjectListResponse {
  const factory ProjectListResponse({
    @Default([]) List<dynamic> projects,
    @Default(0) int total,
    @Default(20) int limit,
    @Default(0) int offset,
  }) = _ProjectListResponse;

  const ProjectListResponse._();

  factory ProjectListResponse.fromJson(Map<String, dynamic> json) =>
      _$ProjectListResponseFromJson(json);
}

/// Request для создания проекта
@freezed
abstract class CreateProjectRequest with _$CreateProjectRequest {
  const factory CreateProjectRequest({
    required String name,
    @Default('') String description,
    @JsonKey(name: 'git_provider')
    required String gitProvider,
    @JsonKey(name: 'git_url')
    required String gitUrl,
    @JsonKey(name: 'git_default_branch')
    @Default('main')
    String gitDefaultBranch,
    @JsonKey(name: 'git_credential_id')
    String? gitCredentialId,
    @JsonKey(name: 'vector_collection')
    required String vectorCollection,
    @JsonKey(name: 'tech_stack')
    @Default({})
    Map<String, dynamic> techStack,
    @Default('active') String status,
    @Default({}) Map<String, dynamic> settings,
  }) = _CreateProjectRequest;

  const CreateProjectRequest._();

  factory CreateProjectRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateProjectRequestFromJson(json);
}

/// Request для обновления проекта (partial update)
@freezed
abstract class UpdateProjectRequest with _$UpdateProjectRequest {
  const factory UpdateProjectRequest({
    String? name,
    String? description,
    @JsonKey(name: 'git_provider')
    String? gitProvider,
    @JsonKey(name: 'git_url')
    String? gitUrl,
    @JsonKey(name: 'git_default_branch')
    String? gitDefaultBranch,
    @JsonKey(name: 'git_credential_id')
    String? gitCredentialId,
    @JsonKey(name: 'vector_collection')
    String? vectorCollection,
    String? status,
    @JsonKey(name: 'tech_stack')
    Map<String, dynamic>? techStack,
    Map<String, dynamic>? settings,
    @JsonKey(name: 'remove_git_credential')
    @Default(false)
    bool removeGitCredential,
    @JsonKey(name: 'clear_tech_stack')
    @Default(false)
    bool clearTechStack,
    @JsonKey(name: 'clear_settings')
    @Default(false)
    bool clearSettings,
  }) = _UpdateProjectRequest;

  const UpdateProjectRequest._();

  factory UpdateProjectRequest.fromJson(Map<String, dynamic> json) =>
      _$UpdateProjectRequestFromJson(json);
}
