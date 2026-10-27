using System.Text.Json;

namespace Moonwell.GameData;

/// <summary>
/// The game's files by the paths object data writes (<c>ReplaceableTextures\CommandButtons\BTNPeasant.blp</c>,
/// <c>units\human\Peasant\Peasant</c>), found in the install's storage as the game finds them: the Classic (SD) files,
/// the installed locale's first (Sven, 2026-10-27; moved here from Moonwell Editor, 2026-10-28); any case, either slash; a model with no extension or <c>.mdl</c> as its
/// <c>.mdx</c>, a <c>.blp</c> or <c>.tif</c> texture as its <c>.dds</c> (most are stored as DDS); and the game's
/// <c>FileAliases.json</c>, which moves some textures (Sven, 2026-10-27). A file only the Reforged tiers hold (the
/// <c>_HD</c>, then the <c>_DE</c> files: <c>BTNPriestV1</c>, the icon the data gives the Priest) is read from there.
/// </summary>
public sealed class GameFiles : IDisposable
{
    private const string Root = "War3.w3mod:";
    private const string Aliases = "filealiases.json";

    private const string Hd = Root + "_HD.w3mod:";
    private const string De = Root + "_DE.w3mod:";

    private readonly CascStorage _storage;
    private readonly Dictionary<string, string> _aliases = new(StringComparer.OrdinalIgnoreCase);

    // Where a file is looked for in each graphics mode (Sven, 2026-10-27): the mode's own tier first (Classic is the
    // base files and the installed locale's), then the rest, so a file only another tier holds is still found.
    private readonly Dictionary<string, string[]> _prefixes;

    private GameFiles(CascStorage storage)
    {
        _storage = storage;
        var locale = $"{Root}_Locales/{storage.Build.Locale}.w3mod:";
        _prefixes = new Dictionary<string, string[]>(StringComparer.Ordinal)
        {
            ["sd"] = [locale, Root, Hd, De],
            ["hd"] = [Hd, locale, Root, De],
            ["de"] = [De, locale, Root, Hd],
        };
        if (storage.Read(Root + Aliases) is { } json)
        {
            try
            {
                foreach (var alias in JsonDocument.Parse(json).RootElement.EnumerateArray())
                {
                    if (alias.TryGetProperty("src", out var from) && alias.TryGetProperty("dest", out var to))
                    {
                        _aliases.TryAdd(Normalized(from.GetString() ?? ""), Normalized(to.GetString() ?? ""));
                    }
                }
            }
            catch (JsonException)
            {
                // Without the aliases a moved texture is not found; every other file still is.
            }
        }
    }

    /// <summary>The files of the install in <paramref name="gameFolder"/>.</summary>
    /// <exception cref="IOException">There is no install there, or its storage cannot be read.</exception>
    public static GameFiles Open(string gameFolder) => new(CascStorage.Open(gameFolder));

    /// <summary>The install's active build: its version, build key and locale.</summary>
    public GameBuild Build => _storage.Build;

    /// <summary>The storage itself, with every module's files (<see cref="GameTables"/> reads the tables from it).</summary>
    public CascStorage Storage => _storage;

    /// <summary>
    /// The file at a path as object data writes it, in a graphics mode (<c>sd</c> Classic, <c>hd</c> Reforged, <c>de</c>
    /// Definitive Edition; Classic when not given); null when the game has no such file.
    /// </summary>
    public byte[]? Read(string path, string mode = "sd") => Find(path, mode) is { } found ? _storage.Read(found) : null;

    /// <summary>The storage path the game reads for <paramref name="path"/> in a graphics mode; null when it has none.</summary>
    public string? Find(string path, string mode = "sd")
    {
        var prefixes = _prefixes.GetValueOrDefault(mode, _prefixes["sd"]);
        var normalized = Normalized(path);
        if (normalized.Length == 0)
        {
            return null;
        }

        foreach (var candidate in Candidates(normalized).Concat(
                     _aliases.TryGetValue(normalized, out var moved) ? Candidates(moved) : []))
        {
            foreach (var prefix in prefixes)
            {
                if (_storage.Contains(prefix + candidate))
                {
                    return prefix + candidate;
                }
            }
        }

        return null;
    }

    /// <summary>
    /// The files under <paramref name="folder"/> (<c>ReplaceableTextures/CommandButtons/</c>) whose names end in one
    /// of <paramref name="extensions"/>: the Classic files and, in another graphics mode, its tier's too (where
    /// <c>BTNPriestV1</c> is), as paths object data would write (no storage prefix), once each, sorted.
    /// </summary>
    public List<string> List(string folder, string mode, params string[] extensions)
    {
        var tiers = mode switch
        {
            "hd" => new[] { Root, Hd },
            "de" => [Root, De],
            _ => [Root],
        };
        var found = new SortedSet<string>(StringComparer.OrdinalIgnoreCase);
        foreach (var tier in tiers)
        {
            var start = tier + Normalized(folder);
            foreach (var path in _storage.Paths)
            {
                // Another tier and the locales sit in nested folders (War3.w3mod:_HD.w3mod:...), which are left out.
                if (path.StartsWith(start, StringComparison.OrdinalIgnoreCase)
                    && path.IndexOf(".w3mod:", tier.Length, StringComparison.OrdinalIgnoreCase) < 0
                    && extensions.Any(extension => path.EndsWith(extension, StringComparison.OrdinalIgnoreCase)))
                {
                    found.Add(path[tier.Length..]);
                }
            }
        }

        return [.. found];
    }

    /// <inheritdoc />
    public void Dispose() => _storage.Dispose();

    /// <summary>A path with forward slashes, no leading slash or storage prefix, and no surrounding blanks.</summary>
    public static string Normalized(string path)
    {
        var normalized = path.Trim().Replace('\\', '/').TrimStart('/');
        return normalized.StartsWith(Root, StringComparison.OrdinalIgnoreCase) ? normalized[Root.Length..] : normalized;
    }

    // The path, then the extensions the game falls back to.
    private static IEnumerable<string> Candidates(string path)
    {
        yield return path;
        var extension = Path.GetExtension(path).ToLowerInvariant();
        var stem = extension.Length > 0 ? path[..^extension.Length] : path;
        switch (extension)
        {
            case "":
            case ".mdl":
                yield return stem + ".mdx";
                break;
            case ".blp":
            case ".tif":
            case ".tga":
                yield return stem + ".dds";
                break;
            case ".dds":
                yield return stem + ".blp";
                break;
        }
    }
}
