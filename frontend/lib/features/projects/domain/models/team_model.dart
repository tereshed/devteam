import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';

part 'team_model.freezed.dart';
part 'team_model.g.dart';

@freezed
abstract class TeamModel with _$TeamModel {
  const factory TeamModel({
    /// UUID команды
    required String id,

    /// Название команды
    required String name,

    /// ID проекта, которому принадлежит команда
    @JsonKey(name: 'project_id')
    required String projectId,

    /// Тип команды (например, 'development')
    required String type,

    /// Список агентов в команде
    required List<AgentModel> agents,

    /// Дата создания
    @JsonKey(name: 'created_at')
    required DateTime createdAt,

    /// Дата последнего обновления
    @JsonKey(name: 'updated_at')
    required DateTime updatedAt,
  }) = _TeamModel;

  const TeamModel._();

  factory TeamModel.fromJson(Map<String, dynamic> json) =>
      _$TeamModelFromJson(json);
}
