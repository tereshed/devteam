import 'package:freezed_annotation/freezed_annotation.dart';

part 'team_type_model.freezed.dart';
part 'team_type_model.g.dart';

@freezed
abstract class TeamTypeModel with _$TeamTypeModel {
  const factory TeamTypeModel({
    required String code,
    required String name,
    @JsonKey(name: 'is_system') required bool isSystem,
  }) = _TeamTypeModel;

  factory TeamTypeModel.fromJson(Map<String, dynamic> json) =>
      _$TeamTypeModelFromJson(json);
}
