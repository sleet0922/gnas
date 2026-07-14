import 'dart:io';
import 'package:flutter/material.dart';
import 'package:file_picker/file_picker.dart';
import 'package:path_provider/path_provider.dart';
import 'package:http/http.dart' as http;
import '../services/api_service.dart';
import '../models/file_info.dart';

class FilesPage extends StatefulWidget {
  const FilesPage({super.key});

  @override
  State<FilesPage> createState() => _FilesPageState();
}

class _FilesPageState extends State<FilesPage>
    with AutomaticKeepAliveClientMixin {
  final _api = ApiService();
  final _pathController = TextEditingController(text: '/');
  List<FileInfo> _files = [];
  bool _loading = true;
  String? _error;
  String _currentPath = '/';
  final List<String> _history = ['/'];

  @override
  bool get wantKeepAlive => true;

  @override
  void initState() {
    super.initState();
    _loadFiles();
  }

  @override
  void dispose() {
    _pathController.dispose();
    super.dispose();
  }

  Future<void> _loadFiles() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final res = await _api.listFiles(_currentPath);
    if (!mounted) return;
    if (res.isSuccess && res.data != null) {
      setState(() {
        _files = res.data!;
        _loading = false;
        _pathController.text = _currentPath;
      });
    } else {
      setState(() {
        _error = res.message ?? '加载失败';
        _loading = false;
      });
    }
  }

  void _enterDir(String path) {
    setState(() {
      _currentPath = path;
      _history.add(path);
    });
    _loadFiles();
  }

  void _goBack() {
    if (_history.length > 1) {
      setState(() {
        _history.removeLast();
        _currentPath = _history.last;
      });
      _loadFiles();
    }
  }

  Future<void> _uploadFile() async {
    final result = await FilePicker.platform.pickFiles();
    if (result == null || result.files.single.path == null) return;
    final file = File(result.files.single.path!);
    if (!mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    messenger.showSnackBar(const SnackBar(content: Text('上传中...')));
    final res = await _api.uploadFile(_currentPath, file);
    if (!mounted) return;
    if (res.isSuccess) {
      messenger.showSnackBar(
        SnackBar(
          content: const Text('上传成功'),
          backgroundColor: Colors.green.shade600,
        ),
      );
      _loadFiles();
    } else {
      messenger.showSnackBar(
        SnackBar(
          content: Text(res.message ?? '上传失败'),
          backgroundColor: Colors.red.shade600,
        ),
      );
    }
  }

  Future<void> _createDir() async {
    final controller = TextEditingController();
    final name = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('新建文件夹'),
        content: TextField(
          controller: controller,
          decoration: const InputDecoration(
            hintText: '文件夹名称',
            border: OutlineInputBorder(),
          ),
          autofocus: true,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, controller.text.trim()),
            child: const Text('创建'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (name == null || name.isEmpty) return;
    final path = '${_currentPath == '/' ? '' : _currentPath}/$name';
    final res = await _api.createDir(path);
    if (!mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    if (res.isSuccess) {
      _loadFiles();
    } else {
      messenger.showSnackBar(
        SnackBar(
          content: Text(res.message ?? '创建失败'),
          backgroundColor: Colors.red.shade600,
        ),
      );
    }
  }

  Future<void> _rename(FileInfo file) async {
    final controller = TextEditingController(text: file.name);
    final newName = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('重命名'),
        content: TextField(
          controller: controller,
          decoration: const InputDecoration(border: OutlineInputBorder()),
          autofocus: true,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, controller.text.trim()),
            child: const Text('确定'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (newName == null || newName.isEmpty || newName == file.name) return;
    final res = await _api.renameFile(file.path, newName);
    if (!mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    if (res.isSuccess) {
      _loadFiles();
    } else {
      messenger.showSnackBar(
        SnackBar(
          content: Text(res.message ?? '重命名失败'),
          backgroundColor: Colors.red.shade600,
        ),
      );
    }
  }

  Future<void> _delete(FileInfo file) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('确认删除'),
        content: Text('确定要删除 "${file.name}" 吗？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
            ),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (confirm != true) return;
    final res = await _api.deleteFile(file.path);
    if (!mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    if (res.isSuccess) {
      _loadFiles();
    } else {
      messenger.showSnackBar(
        SnackBar(
          content: Text(res.message ?? '删除失败'),
          backgroundColor: Colors.red.shade600,
        ),
      );
    }
  }

  Future<void> _download(FileInfo file) async {
    try {
      final dir = await getApplicationDocumentsDirectory();
      final savePath = '${dir.path}/${file.name}';
      final url = _api.getDownloadUrl(file.path);
      final response = await http.get(Uri.parse(url));
      if (response.statusCode == 200) {
        await File(savePath).writeAsBytes(response.bodyBytes);
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('已下载到: $savePath'),
            backgroundColor: Colors.green.shade600,
          ),
        );
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('下载失败: $e'),
          backgroundColor: Colors.red.shade600,
        ),
      );
    }
  }

  IconData _getIcon(FileInfo f) {
    if (f.isDir) return Icons.folder;
    if (f.isImage) return Icons.image;
    if (f.isVideo) return Icons.videocam;
    final ext = f.name.split('.').last.toLowerCase();
    switch (ext) {
      case 'pdf':
        return Icons.picture_as_pdf;
      case 'doc':
      case 'docx':
        return Icons.description;
      case 'zip':
      case 'rar':
      case '7z':
        return Icons.folder_zip;
      case 'mp3':
      case 'wav':
      case 'flac':
        return Icons.audio_file;
      case 'txt':
        return Icons.article;
      default:
        return Icons.insert_drive_file;
    }
  }

  String _formatModTime(String value) {
    if (value.length >= 10) return value.substring(0, 10);
    return value.isEmpty ? '-' : value;
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    final theme = Theme.of(context);
    return Column(
      children: [
        // Path bar
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
              if (_history.length > 1)
                IconButton(
                  icon: const Icon(Icons.arrow_back),
                  onPressed: _goBack,
                ),
              Expanded(
                child: Text(
                  _currentPath,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontFamily: 'monospace',
                  ),
                ),
              ),
              IconButton(
                icon: const Icon(Icons.refresh),
                onPressed: _loadFiles,
                tooltip: '刷新',
              ),
              IconButton(
                icon: const Icon(Icons.create_new_folder),
                onPressed: _createDir,
                tooltip: '新建文件夹',
              ),
              IconButton(
                icon: const Icon(Icons.upload),
                onPressed: _uploadFile,
                tooltip: '上传文件',
              ),
            ],
          ),
        ),
        // File list
        Expanded(
          child: _loading
              ? const Center(child: CircularProgressIndicator())
              : _error != null
              ? Center(child: Text(_error!))
              : _files.isEmpty
              ? Center(
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.folder_open,
                        size: 64,
                        color: Colors.grey.shade300,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        '空目录',
                        style: TextStyle(color: Colors.grey.shade500),
                      ),
                    ],
                  ),
                )
              : RefreshIndicator(
                  onRefresh: _loadFiles,
                  child: ListView.separated(
                    itemCount: _files.length,
                    separatorBuilder: (_, _) => const Divider(height: 1),
                    itemBuilder: (_, i) {
                      final f = _files[i];
                      return ListTile(
                        leading: Icon(
                          _getIcon(f),
                          color: f.isDir
                              ? Colors.amber.shade600
                              : theme.colorScheme.primary,
                        ),
                        title: Text(f.name),
                        subtitle: f.isDir
                            ? null
                            : Text(
                                '${f.formattedSize}  •  ${_formatModTime(f.modTime)}',
                                style: theme.textTheme.bodySmall,
                              ),
                        trailing: PopupMenuButton<String>(
                          onSelected: (v) {
                            switch (v) {
                              case 'rename':
                                _rename(f);
                              case 'delete':
                                _delete(f);
                              case 'download':
                                _download(f);
                            }
                          },
                          itemBuilder: (_) => [
                            if (!f.isDir)
                              const PopupMenuItem(
                                value: 'download',
                                child: Text('下载'),
                              ),
                            const PopupMenuItem(
                              value: 'rename',
                              child: Text('重命名'),
                            ),
                            const PopupMenuItem(
                              value: 'delete',
                              child: Text('删除'),
                            ),
                          ],
                        ),
                        onTap: f.isDir
                            ? () => _enterDir(f.path)
                            : f.isImage
                            ? () => _previewImage(f)
                            : null,
                      );
                    },
                  ),
                ),
        ),
      ],
    );
  }

  void _previewImage(FileInfo file) {
    final url = _api.getDownloadUrl(file.path, disposition: 'inline');
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => Scaffold(
          appBar: AppBar(title: Text(file.name)),
          body: InteractiveViewer(
            child: Center(child: Image.network(url, fit: BoxFit.contain)),
          ),
        ),
      ),
    );
  }
}
