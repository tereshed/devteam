import 'package:dio/dio.dart';
import 'package:frontend/features/admin/prompts/domain/prompt_model.dart';

class PromptsRepository {
  final Dio _dio;

  PromptsRepository({required Dio dio}) : _dio = dio;

  Future<List<Prompt>> getPrompts() async {
    final response = await _dio.get('/prompts');
    final List<dynamic> data = response.data as List<dynamic>;
    return data
        .map((json) => Prompt.fromJson(json as Map<String, dynamic>))
        .toList();
  }

  Future<Prompt> getPrompt(String id) async {
    final response = await _dio.get('/prompts/$id');
    return Prompt.fromJson(response.data as Map<String, dynamic>);
  }

  Future<Prompt> createPrompt({
    required String name,
    required String description,
    required String template,
    Map<String, dynamic>? jsonSchema,
    bool isActive = true,
  }) async {
    final response = await _dio.post(
      '/prompts',
      data: {
        'name': name,
        'description': description,
        'template': template,
        'json_schema': jsonSchema,
        'is_active': isActive,
      },
    );
    return Prompt.fromJson(response.data as Map<String, dynamic>);
  }

  Future<Prompt> updatePrompt({
    required String id,
    String? description,
    String? template,
    Map<String, dynamic>? jsonSchema,
    bool? isActive,
  }) async {
    final response = await _dio.put(
      '/prompts/$id',
      data: {
        if (description != null) 'description': description,
        if (template != null) 'template': template,
        if (jsonSchema != null) 'json_schema': jsonSchema,
        if (isActive != null) 'is_active': isActive,
      },
    );
    return Prompt.fromJson(response.data as Map<String, dynamic>);
  }

  Future<void> deletePrompt(String id) async {
    await _dio.delete('/prompts/$id');
  }
}
