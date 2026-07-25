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
  bool _sslEnabled = false;
  bool _savingSSL = false;
  final _sslCertController = TextEditingController(text: '/ssl/1.pem');
  final _sslKeyController = TextEditingController(text: '/ssl/1.key');

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
        _sslEnabled = res.data?['ssl_enabled'] == true;
        _sslCertController.text =
            res.data?['ssl_cert_file'] as String? ?? '/ssl/1.pem';
        _sslKeyController.text =
            res.data?['ssl_key_file'] as String? ?? '/ssl/1.key';
        _loadingSetting = false;
      });
    } else {
      setState(() => _loadingSetting = false);
    }
  }

  Future<void> _toggleAI(bool val) async {
    setState(() => _loadingSetting = true);
    final res = await _api.updateSettings(aiEnabled: val);
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

  Future<void> _saveSSL() async {
    setState(() => _savingSSL = true);
    final res = await _api.updateSettings(
      sslEnabled: _sslEnabled,
      sslCertFile: _sslCertController.text.trim(),
      sslKeyFile: _sslKeyController.text.trim(),
    );
    if (!mounted) return;
    setState(() => _savingSSL = false);
    if (!res.isSuccess) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(res.message ?? '更新 SSL 设置失败')),
      );
      return;
    }

    final restartScheduled = res.data?['restart_scheduled'] == true;
    final restartRequired = res.data?['restart_required'] == true;
    if (restartRequired) {
      final current = Uri.tryParse(_api.baseUrl);
      if (current != null) {
        await _api.saveHost(
          current.replace(scheme: _sslEnabled ? 'https' : 'http').toString(),
        );
      }
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            restartScheduled
                ? 'SSL 设置已保存，服务正在重启，后续请求将使用新的协议'
                : 'SSL 设置已保存，请重启 gnas 服务后使用新的协议',
          ),
          duration: const Duration(seconds: 6),
        ),
      );
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('SSL 设置已保存')),
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
  void dispose() {
    _sslCertController.dispose();
    _sslKeyController.dispose();
    super.dispose();
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
        // SSL Section
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Text(
            'HTTPS / SSL',
            style: theme.textTheme.titleSmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
        Card(
          margin: const EdgeInsets.symmetric(horizontal: 16),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
            child: Column(
              children: [
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  secondary: Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: theme.colorScheme.primaryContainer,
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Icon(
                      Icons.lock_outline,
                      size: 18,
                      color: theme.colorScheme.onPrimaryContainer,
                    ),
                  ),
                  title: const Text('启用 HTTPS'),
                  subtitle: const Text('保存后服务会自动重启并继续使用 8082 端口'),
                  value: _sslEnabled,
                  onChanged: _savingSSL
                      ? null
                      : (value) => setState(() => _sslEnabled = value),
                ),
                TextField(
                  controller: _sslCertController,
                  enabled: !_savingSSL,
                  decoration: const InputDecoration(
                    labelText: '证书文件路径',
                    hintText: '/ssl/1.pem',
                    prefixIcon: Icon(Icons.card_membership_outlined),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _sslKeyController,
                  enabled: !_savingSSL,
                  decoration: const InputDecoration(
                    labelText: '私钥文件路径',
                    hintText: '/ssl/1.key',
                    prefixIcon: Icon(Icons.key_outlined),
                  ),
                ),
                const SizedBox(height: 12),
                Align(
                  alignment: Alignment.centerRight,
                  child: FilledButton.tonalIcon(
                    onPressed: _savingSSL ? null : _saveSSL,
                    icon: _savingSSL
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.save_outlined),
                    label: const Text('保存 SSL 设置'),
                  ),
                ),
              ],
            ),
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
