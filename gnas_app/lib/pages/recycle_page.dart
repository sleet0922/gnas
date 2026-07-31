import 'package:flutter/material.dart';
import '../services/api_service.dart';

class RecyclePage extends StatefulWidget {
  const RecyclePage({super.key});

  @override
  State<RecyclePage> createState() => _RecyclePageState();
}

class _RecyclePageState extends State<RecyclePage> {
  final _api = ApiService();
  List<Map<String, dynamic>> _items = [];
  bool _loading = true;
  String? _error;

  bool _isSelectionMode = false;
  final Set<int> _selectedIds = {};

  @override
  void initState() {
    super.initState();
    _loadRecycleBin();
  }

  int _getId(Map<String, dynamic> item) {
    final id = item['id'];
    if (id is int) return id;
    if (id is num) return id.toInt();
    return int.tryParse('$id') ?? 0;
  }

  Future<void> _loadRecycleBin() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final res = await _api.getRecycleBin();
    if (!mounted) return;
    if (res.isSuccess) {
      setState(() {
        _items = res.data ?? [];
        _loading = false;
      });
    } else {
      setState(() {
        _error = res.message ?? '加载失败';
        _loading = false;
      });
    }
  }

  void _enterSelectionMode(int id) {
    setState(() {
      _isSelectionMode = true;
      _selectedIds.clear();
      _selectedIds.add(id);
    });
  }

  void _exitSelectionMode() {
    setState(() {
      _isSelectionMode = false;
      _selectedIds.clear();
    });
  }

  void _toggleSelection(int id) {
    setState(() {
      if (_selectedIds.contains(id)) {
        _selectedIds.remove(id);
        if (_selectedIds.isEmpty) {
          _isSelectionMode = false;
        }
      } else {
        _selectedIds.add(id);
      }
    });
  }

  void _selectAll() {
    setState(() {
      if (_selectedIds.length == _items.length) {
        _selectedIds.clear();
        _isSelectionMode = false;
      } else {
        _selectedIds.addAll(_items.map(_getId));
      }
    });
  }

  Future<void> _restoreSelected() async {
    if (_selectedIds.isEmpty) return;
    final messenger = ScaffoldMessenger.of(context);
    final ids = _selectedIds.toList();
    final res = await _api.restoreRecycleItems(ids);
    if (!mounted) return;
    if (res.isSuccess) {
      messenger.showSnackBar(
        SnackBar(
          content: Text('已恢复 ${res.data ?? ids.length} 项'),
          backgroundColor: Colors.green.shade600,
        ),
      );
      _exitSelectionMode();
      _loadRecycleBin();
    } else {
      messenger.showSnackBar(
        SnackBar(
          content: Text(res.message ?? '恢复失败'),
          backgroundColor: Colors.red.shade600,
        ),
      );
    }
  }

  Future<void> _deleteSelected() async {
    if (_selectedIds.isEmpty) return;
    final messenger = ScaffoldMessenger.of(context);
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogTheme = Theme.of(ctx);
        return AlertDialog(
          title: const Text('确认彻底删除'),
          content: Text(
            '确定要彻底删除选中的 ${_selectedIds.length} 项吗？此操作不可撤销。',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消'),
            ),
            FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: dialogTheme.colorScheme.error,
              ),
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('彻底删除'),
            ),
          ],
        );
      },
    );
    if (confirm != true) return;

    final ids = _selectedIds.toList();
    final res = await _api.deleteRecycleItems(ids);
    if (!mounted) return;
    if (res.isSuccess) {
      messenger.showSnackBar(
        SnackBar(
          content: Text('已彻底删除 ${res.data ?? ids.length} 项'),
          backgroundColor: Colors.green.shade600,
        ),
      );
      _exitSelectionMode();
      _loadRecycleBin();
    } else {
      messenger.showSnackBar(
        SnackBar(
          content: Text(res.message ?? '删除失败'),
          backgroundColor: Colors.red.shade600,
        ),
      );
    }
  }

  Future<void> _clearAll() async {
    if (_items.isEmpty) return;
    final messenger = ScaffoldMessenger.of(context);
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogTheme = Theme.of(ctx);
        return AlertDialog(
          title: const Text('确认清空回收站'),
          content: const Text('确定要清空回收站吗？所有文件将被彻底删除，此操作不可撤销。'),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消'),
            ),
            FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: dialogTheme.colorScheme.error,
              ),
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('清空'),
            ),
          ],
        );
      },
    );
    if (confirm != true) return;

    final res = await _api.clearRecycleBin();
    if (!mounted) return;
    if (res.isSuccess) {
      messenger.showSnackBar(
        SnackBar(
          content: Text('已清空回收站（${res.data ?? 0} 项）'),
          backgroundColor: Colors.green.shade600,
        ),
      );
      _exitSelectionMode();
      _loadRecycleBin();
    } else {
      messenger.showSnackBar(
        SnackBar(
          content: Text(res.message ?? '清空失败'),
          backgroundColor: Colors.red.shade600,
        ),
      );
    }
  }

  String _formatExpire(String? expireAt) {
    if (expireAt == null || expireAt.isEmpty) return '';
    try {
      final expire = DateTime.parse(expireAt).toLocal();
      final now = DateTime.now();
      final diff = expire.difference(now);
      if (diff.isNegative) return '已过期';
      if (diff.inDays > 0) return '${diff.inDays}天后过期';
      if (diff.inHours > 0) return '${diff.inHours}小时后过期';
      if (diff.inMinutes > 0) return '${diff.inMinutes}分钟后过期';
      return '即将过期';
    } catch (_) {
      return '';
    }
  }

  Widget _placeholder(bool isVideo, bool isDir) {
    return Container(
      color: Colors.grey.shade200,
      child: Icon(
        isDir ? Icons.folder : (isVideo ? Icons.videocam : Icons.image),
        color: Colors.grey.shade400,
        size: 40,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    Widget content;
    if (_loading) {
      content = const Center(child: CircularProgressIndicator());
    } else if (_error != null) {
      content = Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: theme.colorScheme.error),
            const SizedBox(height: 12),
            Text(_error!),
            const SizedBox(height: 12),
            FilledButton.tonal(
              onPressed: _loadRecycleBin,
              child: const Text('重试'),
            ),
          ],
        ),
      );
    } else if (_items.isEmpty) {
      content = Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.delete_outline,
              size: 80,
              color: Colors.grey.shade300,
            ),
            const SizedBox(height: 12),
            Text('回收站为空', style: TextStyle(color: Colors.grey.shade500)),
          ],
        ),
      );
    } else {
      content = RefreshIndicator(
        onRefresh: _loadRecycleBin,
        child: LayoutBuilder(
          builder: (_, constraints) {
            final crossAxisCount = constraints.maxWidth > 600 ? 4 : 3;
            return GridView.builder(
              padding: const EdgeInsets.all(4),
              gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: crossAxisCount,
                crossAxisSpacing: 2,
                mainAxisSpacing: 2,
              ),
              itemCount: _items.length,
              itemBuilder: (_, i) {
                final item = _items[i];
                final id = _getId(item);
                final name = item['name'] as String? ?? '';
                final isVideo = item['isVideo'] == true;
                final isDir = item['isDir'] == true;
                final hasThumb = item['hasThumb'] == true;
                final expireAt = item['expireAt'] as String?;
                final isSelected = _selectedIds.contains(id);

                return GestureDetector(
                  onTap: () {
                    if (_isSelectionMode) {
                      _toggleSelection(id);
                    }
                  },
                  onLongPress: () {
                    if (_isSelectionMode) {
                      _toggleSelection(id);
                    } else {
                      _enterSelectionMode(id);
                    }
                  },
                  child: Stack(
                    fit: StackFit.expand,
                    children: [
                      if (isDir)
                        _placeholder(isVideo, isDir)
                      else if (hasThumb)
                        Image.network(
                          _api.getRecycleThumbUrl(id),
                          fit: BoxFit.cover,
                          errorBuilder: (_, _, _) => _placeholder(
                            isVideo,
                            isDir,
                          ),
                        )
                      else
                        _placeholder(isVideo, isDir),
                      if (isVideo)
                        const Center(
                          child: SizedBox(
                            width: 48,
                            height: 48,
                            child: DecoratedBox(
                              decoration: BoxDecoration(
                                color: Colors.black54,
                                shape: BoxShape.circle,
                              ),
                              child: Icon(
                                Icons.play_arrow,
                                color: Colors.white,
                                size: 32,
                              ),
                            ),
                          ),
                        ),
                      Positioned(
                        bottom: 0,
                        left: 0,
                        right: 0,
                        child: Container(
                          decoration: const BoxDecoration(
                            gradient: LinearGradient(
                              begin: Alignment.topCenter,
                              end: Alignment.bottomCenter,
                              colors: [Colors.transparent, Colors.black54],
                            ),
                          ),
                          padding: const EdgeInsets.symmetric(
                            horizontal: 4,
                            vertical: 2,
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Text(
                                name,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  color: Colors.white,
                                  fontSize: 11,
                                ),
                              ),
                              Text(
                                _formatExpire(expireAt),
                                style: const TextStyle(
                                  color: Colors.white70,
                                  fontSize: 10,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                      if (_isSelectionMode)
                        Positioned(
                          top: 6,
                          right: 6,
                          child: Container(
                            width: 24,
                            height: 24,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              color: isSelected
                                  ? theme.colorScheme.primary
                                  : Colors.transparent,
                              border: Border.all(
                                color: Colors.white,
                                width: 2,
                              ),
                            ),
                            child: isSelected
                                ? const Icon(
                                    Icons.check,
                                    color: Colors.white,
                                    size: 16,
                                  )
                                : null,
                          ),
                        ),
                    ],
                  ),
                );
              },
            );
          },
        ),
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(
          _isSelectionMode ? '已选 ${_selectedIds.length} 项' : '回收站',
        ),
        leading: _isSelectionMode
            ? IconButton(
                icon: const Icon(Icons.close),
                onPressed: _exitSelectionMode,
              )
            : null,
        actions: _isSelectionMode
            ? [
                IconButton(
                  icon: Icon(
                    _selectedIds.length == _items.length
                        ? Icons.deselect
                        : Icons.select_all,
                  ),
                  tooltip: '全选/取消全选',
                  onPressed: _selectAll,
                ),
                IconButton(
                  icon: const Icon(Icons.restore),
                  tooltip: '恢复选中',
                  onPressed: _selectedIds.isEmpty ? null : _restoreSelected,
                ),
                IconButton(
                  icon: const Icon(Icons.delete_forever),
                  tooltip: '彻底删除选中',
                  onPressed: _selectedIds.isEmpty ? null : _deleteSelected,
                ),
              ]
            : [
                IconButton(
                  icon: const Icon(Icons.delete_forever),
                  tooltip: '清空回收站',
                  onPressed: _items.isEmpty ? null : _clearAll,
                ),
                IconButton(
                  icon: const Icon(Icons.refresh),
                  onPressed: _loadRecycleBin,
                ),
              ],
      ),
      body: content,
    );
  }
}
