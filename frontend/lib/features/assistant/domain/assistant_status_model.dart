import 'package:freezed_annotation/freezed_annotation.dart';

part 'assistant_status_model.freezed.dart';
part 'assistant_status_model.g.dart';

@freezed
abstract class AssistantStatusModel with _$AssistantStatusModel {
  const factory AssistantStatusModel({
    @JsonKey(name: 'is_configured') required bool isConfigured,
    @JsonKey(name: 'required_provider') required String requiredProvider,
  }) = _AssistantStatusModel;

  factory AssistantStatusModel.fromJson(Map<String, dynamic> json) =>
      _$AssistantStatusModelFromJson(json);
}