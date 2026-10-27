using System.Text;

namespace Moonwell.GameData;

/// <summary>
/// The files <see cref="GameTables"/> reads, by the paths of an export (<c>units/unitdata.slk</c>,
/// <c>_balance/custom_v1.w3mod/units/unitdata.slk</c>), matched without regard to case: an export folder, or an install's
/// storage.
/// </summary>
internal interface ITableFiles
{
    /// <summary>Where the files are, for messages: the export's folder, or the install's.</summary>
    string Where { get; }

    /// <summary>The file at <paramref name="relative"/>, as the key <see cref="Read"/> takes; null when there is none.</summary>
    string? Find(string relative);

    /// <summary>True when <paramref name="relative"/> is a folder (a balance set's) that holds files.</summary>
    bool HasFolder(string relative);

    /// <summary>The files directly in <paramref name="folder"/>, by name, with the keys <see cref="Read"/> takes; none when it is missing.</summary>
    IEnumerable<(string Name, string Key)> List(string folder);

    /// <summary>The bytes of a file <see cref="Find"/> or <see cref="List"/> gave.</summary>
    byte[] Read(string key);
}

/// <summary>An export: the <c>war3.w3mod</c> folder.</summary>
internal sealed class FolderFiles : ITableFiles
{
    private readonly string _folder;

    public FolderFiles(string folder)
    {
        _folder = folder;
    }

    public string Where => _folder;

    public string? Find(string relative)
    {
        var path = Found(relative);
        return path != null && File.Exists(path) ? path : null;
    }

    public bool HasFolder(string relative)
    {
        return Found(relative) is { } path && Directory.Exists(path);
    }

    public IEnumerable<(string Name, string Key)> List(string folder)
    {
        if (Found(folder) is not { } path || !Directory.Exists(path))
        {
            return Array.Empty<(string, string)>();
        }

        return Directory.EnumerateFiles(path).Select(file => (Path.GetFileName(file), file)).ToList();
    }

    public byte[] Read(string key)
    {
        return File.ReadAllBytes(key);
    }

    /// <summary>Finds a path under the folder, matching each segment without regard to case.</summary>
    private string? Found(string relative)
    {
        var path = _folder;
        foreach (var segment in relative.Split('/'))
        {
            var direct = Path.Combine(path, segment);
            if (File.Exists(direct) || Directory.Exists(direct))
            {
                path = direct;
                continue;
            }

            if (!Directory.Exists(path))
            {
                return null;
            }

            string? match = null;
            foreach (var entry in Directory.EnumerateFileSystemEntries(path))
            {
                if (string.Equals(Path.GetFileName(entry), segment, StringComparison.OrdinalIgnoreCase))
                {
                    match = entry;
                }
            }

            if (match == null)
            {
                return null;
            }

            path = match;
        }

        return path;
    }
}

/// <summary>
/// An install's storage, whose names are an export's paths in the <c>War3.w3mod</c> module, each module (a balance set,
/// a locale) ended by a colon: <c>War3.w3mod:_balance/custom_v1.w3mod:units/unitdata.slk</c>. Only the files stored
/// on this computer are found, so another locale's are not.
/// </summary>
internal sealed class StorageFiles : ITableFiles
{
    private const string Module = ".w3mod";

    private readonly CascStorage _storage;
    private readonly Dictionary<string, bool> _folders = new(StringComparer.OrdinalIgnoreCase);
    private Dictionary<string, List<(string Name, string Key)>>? _listing;

    public StorageFiles(CascStorage storage)
    {
        _storage = storage;
    }

    public string Where => "the install in " + _storage.Folder;

    /// <summary>The storage's name for an export path.</summary>
    public static string StoragePath(string relative)
    {
        var path = new StringBuilder("War3" + Module + ":");
        var segments = relative.Split('/');
        for (var index = 0; index < segments.Length; index++)
        {
            path.Append(segments[index]);
            if (index < segments.Length - 1)
            {
                path.Append(segments[index].EndsWith(Module, StringComparison.OrdinalIgnoreCase) ? ':' : '/');
            }
        }

        return path.ToString();
    }

    public string? Find(string relative)
    {
        var path = StoragePath(relative);
        return _storage.IsStored(path) ? path : null;
    }

    public bool HasFolder(string relative)
    {
        if (!_folders.TryGetValue(relative, out var has))
        {
            var start = StoragePath(relative);
            _folders[relative] = has = _storage.Paths.Any(
                path => path.Length > start.Length
                        && (path[start.Length] == ':' || path[start.Length] == '/')
                        && path.StartsWith(start, StringComparison.OrdinalIgnoreCase));
        }

        return has;
    }

    public IEnumerable<(string Name, string Key)> List(string folder)
    {
        if (_listing == null)
        {
            _listing = new Dictionary<string, List<(string, string)>>(StringComparer.OrdinalIgnoreCase);
            foreach (var path in _storage.Paths)
            {
                var end = path.LastIndexOfAny(new[] { '/', ':' });
                if (!_listing.TryGetValue(path.Substring(0, end), out var files))
                {
                    _listing[path.Substring(0, end)] = files = new List<(string, string)>();
                }

                files.Add((path.Substring(end + 1), path));
            }
        }

        return _listing.TryGetValue(StoragePath(folder), out var listed)
            ? listed.Where(file => _storage.IsStored(file.Key)).ToList()
            : Array.Empty<(string, string)>();
    }

    public byte[] Read(string key)
    {
        return _storage.Read(key) ?? throw new FileNotFoundException($"{key} is not stored in {Where}", key);
    }
}
