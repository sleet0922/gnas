import 'package:flutter/material.dart';
import '../services/api_service.dart';
import 'change_password_page.dart';
import 'login_page.dart';
import 'logs_page.dart';

class SettingsPage extends StatefulWidget {
  const SettingsPage({super.key});

  @override
  State<SettingsPage> createState() => _SettingsPageState();
}

class _SettingsPageState extends State<SettingsPage>
    with AutomaticKeepAliveClientMixin {
  final _api = ApiService();
  bool _aiEnabled = false;
  bool _loadingSetting = true;

  @override
  bool get wantKeepAlive => true;

  @override
  void initState() {
    super.initState();
    _loadSettings();
  }

  Future<void> _loadSettings() async {
    final res = await _api.getSettings();
    if (!mounted) return;
    if (res.isSuccess) {
      setState(() {
        _aiEnabled = res.data?['ai_enabled'] == true;
        _loadingSetting = false;
      });
    } else {
      setState(() => _loadingSetting = false);
    }
  }

  Future<void> _toggleAI(bool val) async {
    setState(() => _loadingSetting = true);
    final res = await _api.updateSettings(val);
    if (!mounted) return;
    if (res.isSuccess) {
      setState(() {
        _aiEnabled = val;
        _loadingSetting = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(val ? 'AI 服务正在拉起，后台正配置大模型依赖...' : '已关闭 AI 服务以释放内存')),
      );
    } else {
      setState(() => _loadingSetting = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(res.message ?? '更新设置失败')),
      );
    }
  }

  Future<void> _logout() async {
    final nav = Navigator.of(context);
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('退出登录'),
        content: const Text('确定要退出当前账号吗？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(ctx).colorScheme.error,
            ),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('退出'),
          ),
        ],
      ),
    );
    if (confirm != true) return;

    await _api.logout();
    if (!mounted) return;
    nav.pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const LoginPage()),
      (route) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    final theme = Theme.of(context);

    return ListView(
      padding: const EdgeInsets.symmetric(vertical: 8),
      children: [
        const SizedBox(height: 8),
        // Account section
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Text(
            '账号',
            style: theme.textTheme.titleSmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
        Card(
          margin: const EdgeInsets.symmetric(horizontal: 16),
          child: Column(
            children: [
              ListTile(
                leading: Container(
                  padding: const EdgeInsets.all(8),
                  decoration: BoxDecoration(
                    color: theme.colorScheme.primaryContainer,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Icon(
                    Icons.password,
                    size: 18,
                    color: theme.colorScheme.onPrimaryContainer,
                  ),
                ),
                title: const Text('修改密码'),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  Navigator.of(context).push(
                    MaterialPageRoute(
                      builder: (_) => const ChangePasswordPage(),
                    ),
                  );
                },
              ),
              const Divider(height: 1, indent: 56),
              ListTile(
                leading: Container(
                  padding: const EdgeInsets.all(8),
                  decoration: BoxDecoration(
                    color: theme.colorScheme.tertiaryContainer,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Icon(
                    Icons.article,
                    size: 18,
                    color: theme.colorScheme.onTertiaryContainer,
                  ),
                ),
                title: const Text('运行日志'),
                trailing: const Icon(Icons.chevron_right),
                onTap: () {
                  Navigator.of(
                    context,
                  ).push(MaterialPageRoute(builder: (_) => const LogsPage()));
                },
              ),
              const Divider(height: 1, indent: 56),
              ListTile(
                leading: Container(
                  padding: const EdgeInsets.all(8),
                  decoration: BoxDecoration(
                    color: theme.colorScheme.errorContainer,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Icon(
                    Icons.logout,
                    size: 18,
                    color: theme.colorScheme.error,
                  ),
                ),
                title: const Text('退出登录'),
                trailing: const Icon(Icons.chevron_right),
                onTap: _logout,
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),
        // AI Section
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Text(
            'AI 智能功能',
            style: theme.textTheme.titleSmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
        Card(
          margin: const EdgeInsets.symmetric(horizontal: 16),
          child: SwitchListTile(
            secondary: Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: theme.colorScheme.primaryContainer,
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(
                Icons.psychology,
                size: 18,
                color: theme.colorScheme.onPrimaryContainer,
              ),
            ),
            title: const Text('启用 AI 搜索与图片查重'),
            subtitle: const Text('自动开启向量数据库与大模型以支持语义搜索及相似照片查重'),
            value: _aiEnabled,
            onChanged: _loadingSetting ? null : _toggleAI,
          ),
        ),
        const SizedBox(height: 24),
        // About section
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Text(
            '关于',
            style: theme.textTheme.titleSmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
        Card(
          margin: const EdgeInsets.symmetric(horizontal: 16),
          child: ListTile(
            leading: Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: theme.colorScheme.secondaryContainer,
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(
                Icons.dns,
                size: 18,
                color: theme.colorScheme.onSecondaryContainer,
              ),
            ),
            title: const Text('GNAS'),
            subtitle: const Text('网络附加存储'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              showAboutDialog(
                context: context,
                applicationName: 'GNAS',
                applicationVersion: '1.0.0',
                applicationLegalese: 'GNAS 网络附加存储客户端',
                children: [
                  const SizedBox(height: 8),
                  const Text('基于 Flutter 构建  ·  Material Design 3'),
                ],
              );
            },
          ),
        ),
      ],
    );
  }
}
