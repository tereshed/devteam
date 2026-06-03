import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/webhooks/data/webhook_repository.dart';
import 'package:frontend/features/webhooks/domain/models/webhook_model.dart';

final webhookRepositoryProvider = Provider<WebhookRepository>((ref) {
  final dio = ref.watch(dioClientProvider);
  return WebhookRepository(dio: dio);
});

final webhooksProvider = FutureProvider.autoDispose<List<WebhookModel>>((ref) async {
  final repo = ref.watch(webhookRepositoryProvider);
  return repo.listWebhooks();
});

final projectWebhooksProvider = Provider.family.autoDispose<AsyncValue<List<WebhookModel>>, String>((ref, projectId) {
  final asyncAll = ref.watch(webhooksProvider);
  return asyncAll.whenData((webhooks) => webhooks.where((w) => w.projectId == projectId).toList());
});

final globalWebhooksProvider = Provider.autoDispose<AsyncValue<List<WebhookModel>>>((ref) {
  final asyncAll = ref.watch(webhooksProvider);
  return asyncAll.whenData((webhooks) => webhooks.where((w) => w.projectId == null).toList());
});
