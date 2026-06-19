import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/presentation/utils/branch_template_preview.dart';

void main() {
  test('пустой шаблон → дефолт task/<short>-<slug>', () {
    expect(branchTemplatePreview(''), 'task/a1b2c3d4-fix-login-bug');
  });

  test('командная конвенция issue/{ticket}_{slug} с авто-суффиксом', () {
    expect(
      branchTemplatePreview('issue/{ticket}_{slug}'),
      'issue/DEV-123_fix-login-bug-a1b2c3d4',
    );
  });

  test('явный {short_id} подавляет авто-суффикс', () {
    expect(
      branchTemplatePreview('feature/{short_id}-{slug}'),
      'feature/a1b2c3d4-fix-login-bug',
    );
  });

  test('пустой ticket → fallback на short_id, пустой сегмент схлопывается', () {
    expect(
      branchTemplatePreview('issue/{ticket}_{slug}', ticket: ''),
      'issue/fix-login-bug-a1b2c3d4',
    );
  });

  test('fallback-синтаксис {ticket|short_id} при пустом ticket', () {
    expect(
      branchTemplatePreview('issue/{ticket|short_id}-{slug}', ticket: ''),
      'issue/a1b2c3d4-fix-login-bug-a1b2c3d4',
    );
  });
}
