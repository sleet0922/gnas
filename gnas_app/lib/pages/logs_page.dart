import 'package:flutter/material.dart';
import '../services/api_service.dart';

class LogsPage extends StatefulWidget {
  const LogsPage({super.key});

  @override
  State<LogsPage> createState() => _LogsPageState();
}

class _LogsPageState extends State<LogsPage>
    with AutomaticKeepAliveClientMixin {
  final _api = ApiService();
  List<String> _logs = [];
  bool _loading = true;
  String? _error;

  @override
  bool get wantKeepAlive => true;

  @override
  void initState() {
    super.initState();
    _loadLogs();
  }

  Future<void> _loadLogs() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final res = await _api.getLogs();
    if (!mounted) return;
    if (res.isSuccess && res.data != null) {
      setState(() {
        _logs = res.data!;
        _loading = false;
      });
    } else {
      setState(() {
        _error = res.message ?? '加载失败';
        _loading = false;
      });
    }
  }

  Future<void> _clearLogs() async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('确认清除'),
        content: const Text('确定要清除所有日志吗？'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消')),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('清除'),
          ),
        ],
      ),
    );
    if (confirm != true) return;
    final res = await _api.clearLogs();
    if (!mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    if (res.isSuccess) {
      _loadLogs();
    } else {
      messenger.showSnackBar(
        SnackBar(
          content: Text(res.message ?? '清除失败'),
          backgroundColor: Colors.red.shade600,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    final theme = Theme.of(context);

    return Column(
      children: [
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          decoration: BoxDecoration(
            color: theme.colorScheme.surfaceContainerLow,
            border: Border(
              bottom: BorderSide(color: theme.colorScheme.outlineVariant),
            ),
          ),
          child: Row(
            children: [
              Icon(Icons.article, color: theme.colorScheme.primary),
              const SizedBox(width: 8),
              Text('运行日志', style: theme.textTheme.titleSmall),
              const Spacer(),
              Text('${_logs.length} 条',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  )),
              const SizedBox(width: 8),
              IconButton(
                icon: const Icon(Icons.refresh),
                onPressed: _loadLogs,
                tooltip: '刷新',
              ),
              IconButton(
                icon: const Icon(Icons.delete_sweep),
                onPressed: _clearLogs,
                tooltip: '清除日志',
              ),
            ],
          ),
        ),
        Expanded(
          child: _loading
              ? const Center(child: CircularProgressIndicator())
              : _error != null
                  ? Center(child: Text(_error!))
                  : _logs.isEmpty
                      ? Center(
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(Icons.inbox_outlined,
                                  size: 64, color: Colors.grey.shade300),
                              const SizedBox(height: 8),
                              Text('暂无日志',
                                  style:
                                      TextStyle(color: Colors.grey.shade500)),
                            ],
                          ),
                        )
                      : RefreshIndicator(
                          onRefresh: _loadLogs,
                          child: ListView.separated(
                            padding: const EdgeInsets.all(8),
                            itemCount: _logs.length,
                            separatorBuilder: (_, _) =>
                                const Divider(height: 1),
                            itemBuilder: (_, i) {
                              final log = _logs[i];
                              final isError = log.contains('错误') ||
                                  log.contains('失败') ||
                                  log.contains('Error');
                              final isSuccess = log.contains('成功') ||
                                  log.contains('Success');
                              return ListTile(
                                dense: true,
                                leading: Icon(
                                  isError
                                      ? Icons.error
                                      : isSuccess
                                          ? Icons.check_circle
                                          : Icons.info,
                                  size: 16,
                                  color: isError
                                      ? theme.colorScheme.error
                                      : isSuccess
                                          ? Colors.green
                                          : Colors.grey,
                                ),
                                title: Text(
                                  log,
                                  style: theme.textTheme.bodySmall?.copyWith(
                                    fontFamily: 'monospace',
                                    fontSize: 12,
                                  ),
                                ),
                              );
                            },
                          ),
                        ),
        ),
      ],
    );
  }
}