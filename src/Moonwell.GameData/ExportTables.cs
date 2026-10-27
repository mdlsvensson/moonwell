using System.Text;
using War3Net.IO.Slk;

namespace Moonwell.GameData;

/// <summary>The seven kinds of standard game object.</summary>
public enum Kind
{
    Unit,
    Item,
    Destructable,
    Doodad,
    Ability,
    Buff,
    Upgrade,
}

/// <summary>A row of a metadata table by its column names; a column the row leaves empty is absent.</summary>
public sealed record MetaRow(string Id, IReadOnlyDictionary<string, string> Columns);

/// <summary>One of the values World Editor offers for a metadata type, from a section of <c>ui/uniteditordata.txt</c>.</summary>
/// <param name="Index">The entry's number in its section: <c>00</c> is 0.</param>
/// <param name="Text">The value as the game's data spells it (<c>air</c>, <c>3</c>).</param>
/// <param name="Label">The editor's label key for the value (<c>WESTRING_UE_TARGETTYPE_AIR</c>), or empty when the entry has none.</param>
/// <param name="Alternates">The other spellings the game's data uses for the value (<c>NN_Alt</c>: <c>aliv</c> for <c>alive</c>); empty when none.</param>
public sealed record ChoiceEntry(int Index, string Text, string Label, IReadOnlyList<string> Alternates);

/// <summary>Every column and every key=value of one object, keys as the tables spell them.</summary>
/// <param name="Id">The object id.</param>
/// <param name="Values">The values by key.</param>
/// <param name="Wrapped">
/// The keys whose last text-file value was wrapped in one pair of quotes, which <paramref name="Values"/> has
/// stripped: such a value is one entry even where it holds commas. Null when none was.
/// </param>
public sealed record ObjectRows(string Id, IReadOnlyDictionary<string, string> Values, IReadOnlySet<string>? Wrapped = null);

/// <summary>
/// The game's data export (the <c>war3.w3mod</c> folder): metadata tables, per-object tables and text, and the
/// editor's labels, read as strings exactly as the files spell them.
/// </summary>
/// <remarks>
/// <para>
/// An object's values are merged in a fixed order, a later source replacing an earlier one on the same key: the
/// kind's tables in the order of <see cref="Sources"/>, then its func files, skin files and locale strings. The
/// objects are the ids of the kind's first table (<c>unitdata.slk</c> for units); a row or text section of any other
/// id is dropped, such as the stray <c>nrmf</c> row of <c>unitweapons.slk</c>. A text section is the object's
/// whatever its case, as the game and World Editor read it: the Command Aura buff <c>BOac</c> has its strings under
/// <c>[Boac]</c>. A section that spells an id exactly goes to that id first.
/// </para>
/// <para>
/// A balance set (<c>melee_v0</c>, <c>custom_v0</c>, <c>custom_v1</c>) holds whole replacement files: a file the
/// set has replaces the base's file of the same name, a file it lacks is read from the base.
/// </para>
/// <para>
/// SLK files are read with <c>War3Net.IO.Slk</c> in its keep-text mode (<c>new SylkParser(keepText: true)</c>, in Moonwell's
/// fork, <c>Moonwell.War3Net.IO.Slk</c>); the default parser types each cell: it narrows decimals to a float
/// (<c>19.7984999999999</c> becomes <c>19.7985</c>) and turns <c>#VALUE!</c> and <c>#REF!</c> into 0, by design.
/// The table's <c>Rows</c> and <c>Columns</c> are the highest indexes holding a cell, not counts.
/// </para>
/// </remarks>
public sealed class ExportTables
{
    private static readonly string[] BalanceSets = { "melee_v0", "custom_v0", "custom_v1" };

    private static readonly TextFiles UnitText = new(
        new[] { "units/*unitfunc.txt", "units/unitweaponsfunc.txt", "units/unitskin.txt", "units/unitweaponsskin.txt" },
        new[] { "units/*unitstrings.txt", "units/unitskinstrings.txt" });

    private static readonly TextFiles AbilityText = new(
        new[] { "units/*abilityfunc.txt", "units/commandfunc.txt", "units/abilityskin.txt" },
        new[]
        {
            "units/*abilitystrings.txt", "units/abilityskinstrings.txt", "units/commandstrings.txt",
            "units/commandskinstrings.txt",
        });

    private static readonly Dictionary<Kind, Source> Sources = new()
    {
        [Kind.Unit] = new Source(
            "units/unitmetadata.slk",
            new[] { "units/unitdata.slk", "units/unitbalance.slk", "units/unitui.slk", "units/unitweapons.slk", "units/unitabilities.slk" },
            UnitText),
        [Kind.Item] = new Source(
            "units/unitmetadata.slk",
            new[] { "units/itemdata.slk" },
            new TextFiles(new[] { "units/itemfunc.txt", "units/itemskin.txt" }, new[] { "units/itemstrings.txt", "units/itemskinstrings.txt" })),
        [Kind.Destructable] = new Source(
            "units/destructablemetadata.slk",
            new[] { "units/destructabledata.slk" },
            new TextFiles(new[] { "units/destructableskin.txt" }, new[] { "units/destructableskinstrings.txt" })),
        [Kind.Doodad] = new Source(
            "doodads/doodadmetadata.slk",
            new[] { "doodads/doodads.slk" },
            new TextFiles(new[] { "doodads/doodadskins.txt" }, new[] { "doodads/doodadskinstrings.txt" })),
        [Kind.Ability] = new Source("units/abilitymetadata.slk", new[] { "units/abilitydata.slk" }, AbilityText),
        [Kind.Buff] = new Source("units/abilitybuffmetadata.slk", new[] { "units/abilitybuffdata.slk" }, AbilityText),
        [Kind.Upgrade] = new Source(
            "units/upgrademetadata.slk",
            new[] { "units/upgradedata.slk" },
            new TextFiles(
                new[] { "units/*upgradefunc.txt", "units/upgradeskin.txt" },
                new[] { "units/*upgradestrings.txt", "units/upgradeskinstrings.txt" })),
    };

    private const string LocaleFolder = "_locales/enus.w3mod";
    private const string LabelsFile = LocaleFolder + "/ui/worldeditstrings.txt";
    private const string GameLabelsFile = LocaleFolder + "/ui/worldeditgamestrings.txt";
    private const string LabelsSection = "WorldEditStrings";
    private const string ChoicesFile = "ui/uniteditordata.txt";

    private readonly string _folder;
    private readonly Dictionary<string, string> _labels;
    private readonly Dictionary<Kind, IReadOnlyList<MetaRow>> _meta = new();
    private readonly Dictionary<(Kind, string?), IReadOnlyList<ObjectRows>> _objects = new();
    private Dictionary<string, IReadOnlyList<ChoiceEntry>>? _choices;

    private ExportTables(string folder, Dictionary<string, string> labels)
    {
        _folder = folder;
        _labels = labels;
    }

    /// <summary>
    /// Opens the export in <paramref name="folder"/> (the <c>war3.w3mod</c> folder) and reads its labels: the editor's
    /// (<c>worldeditstrings.txt</c>), then those only the game uses (<c>worldeditgamestrings.txt</c>, the destructables'
    /// names among them).
    /// </summary>
    public static ExportTables Read(string folder)
    {
        var tables = new ExportTables(folder, new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase));
        tables.ReadLabels(LabelsFile);
        tables.ReadLabels(GameLabelsFile);
        return tables;
    }

    /// <summary>
    /// Adds the labels of a locale file; a key an earlier file has keeps its label, and so does a key given twice in one
    /// file: <c>WESTRING_UE_TARGETTYPE_SELF</c> is "Self", and later "Shelf".
    /// </summary>
    private void ReadLabels(string relative)
    {
        var text = ReadSections(Require(relative, null), out _, firstWins: true);
        if (text.TryGetValue(LabelsSection, out var section))
        {
            foreach (var label in section)
            {
                _labels.TryAdd(label.Key, label.Value);
            }
        }
    }

    /// <summary>
    /// The rows of the kind's metadata table. Items are fields of the unit metadata table, told apart by its
    /// <c>useItem</c> column, so <see cref="Kind.Unit"/> and <see cref="Kind.Item"/> return the same rows.
    /// </summary>
    public IReadOnlyList<MetaRow> Meta(Kind kind)
    {
        if (!_meta.TryGetValue(kind, out var rows))
        {
            var table = ReadTable(Require(Sources[kind].Meta, null), Sources[kind].Meta);
            var list = new List<MetaRow>();
            foreach (var row in table)
            {
                list.Add(new MetaRow(row.Id, row.Cells));
            }

            _meta[kind] = rows = list;
        }

        return rows;
    }

    /// <summary>
    /// The kind's objects with every table and text value merged. <paramref name="set"/> is null for the base tables,
    /// else <c>melee_v0</c>, <c>custom_v0</c> or <c>custom_v1</c>.
    /// </summary>
    public IReadOnlyList<ObjectRows> Objects(Kind kind, string? set)
    {
        if (set != null && Array.IndexOf(BalanceSets, set) < 0)
        {
            throw new ArgumentException(
                $"'{set}' is not a balance set; the sets are {string.Join(", ", BalanceSets)}.", nameof(set));
        }

        if (!_objects.TryGetValue((kind, set), out var objects))
        {
            _objects[(kind, set)] = objects = ReadObjects(Sources[kind], set);
        }

        return objects;
    }

    /// <summary>
    /// The balance sets the export holds, in the order <c>melee_v0</c>, <c>custom_v0</c>, <c>custom_v1</c> (the three
    /// the game has; a set whose folder is missing from <c>_balance</c> is left out).
    /// </summary>
    public IReadOnlyList<string> Sets
    {
        get
        {
            var sets = new List<string>();
            foreach (var set in BalanceSets)
            {
                if (Find(_folder, "_balance/" + set + ".w3mod") != null)
                {
                    sets.Add(set);
                }
            }

            return sets;
        }
    }

    /// <summary>
    /// The ids of the kind's objects that exist in the game under the balance set: an object whose row every table
    /// the set replaces holds. A set's tables are whole tables and an object one of them leaves out does not exist
    /// (the game gives it 0 hit points), so it is not here even when the base tables hold it. An id the set's tables
    /// hold and the base tables do not is here.
    /// </summary>
    public IReadOnlySet<string> Present(Kind kind, string set)
    {
        var present = new HashSet<string>(StringComparer.Ordinal);
        foreach (var listed in Objects(kind, set))
        {
            present.Add(listed.Id);
        }

        var setFolder = SetFolder(set);
        foreach (var path in Sources[kind].Tables)
        {
            var file = Find(setFolder, path);
            if (file == null)
            {
                continue;
            }

            var ids = new HashSet<string>(StringComparer.Ordinal);
            foreach (var row in ReadTable(file, path))
            {
                ids.Add(row.Id);
            }

            present.IntersectWith(ids);
        }

        return present;
    }

    /// <summary>
    /// The gameplay constants of the <c>[Misc]</c> section of <c>units/miscgame.txt</c> (<c>StrHitPointBonus</c>,
    /// <c>IntManaBonus</c>), by key as the file spells them. <paramref name="set"/> is null for the base file, else
    /// <c>melee_v0</c>, <c>custom_v0</c> or <c>custom_v1</c>, whose own file replaces the base's when it has one.
    /// </summary>
    public IReadOnlyDictionary<string, string> Misc(string? set)
    {
        const string path = "units/miscgame.txt";
        var sections = ReadSections(Require(path, set), out _);
        return sections.TryGetValue("Misc", out var misc)
            ? misc
            : throw new FormatException($"{path} has no [Misc] section");
    }

    /// <summary>The English label for a <c>WESTRING_</c> key (looked up without regard to case, as the game does), or the key itself when the export has none.</summary>
    public string Label(string key)
    {
        return _labels.TryGetValue(key, out var label) ? label : key;
    }

    /// <summary>
    /// The values World Editor offers for a metadata type (<c>attackType</c>, <c>targetList</c>), in the order of their
    /// numbers, from the section of <c>ui/uniteditordata.txt</c> the type names; null when the file has no section for
    /// it. An entry is <c>NN=text,label[,version]</c>, and <c>NN_Alt=one,two</c> gives the other spellings of entry
    /// <c>NN</c>; the section's other keys (<c>Sort</c>, <c>NumValues</c>) are not values.
    /// </summary>
    public IReadOnlyList<ChoiceEntry>? Choices(string type)
    {
        _choices ??= ReadChoices(Require(ChoicesFile, null));
        return _choices.TryGetValue(type, out var entries) ? entries : null;
    }

    private static Dictionary<string, IReadOnlyList<ChoiceEntry>> ReadChoices(string path)
    {
        var choices = new Dictionary<string, IReadOnlyList<ChoiceEntry>>(StringComparer.Ordinal);
        foreach (var section in ReadSections(path, out _))
        {
            var entries = new SortedDictionary<int, (string Text, string Label)>();
            var alternates = new Dictionary<int, List<string>>();
            foreach (var (key, value) in section.Value)
            {
                var alternate = key.EndsWith("_Alt", StringComparison.Ordinal);
                var digits = alternate ? key.Substring(0, key.Length - 4) : key;
                if (digits.Length == 0 || !digits.All(char.IsAsciiDigit))
                {
                    continue;
                }

                var index = int.Parse(digits, System.Globalization.CultureInfo.InvariantCulture);
                var parts = value.Split(',');
                if (alternate)
                {
                    alternates[index] = parts.Select(p => p.Trim()).Where(p => p.Length > 0).ToList();
                }
                else
                {
                    entries[index] = (parts[0].Trim(), parts.Length > 1 ? parts[1].Trim() : "");
                }
            }

            if (entries.Count == 0)
            {
                continue;
            }

            var list = new List<ChoiceEntry>(entries.Count);
            foreach (var (index, entry) in entries)
            {
                list.Add(new ChoiceEntry(index, entry.Text, entry.Label, alternates.TryGetValue(index, out var other) ? other : new List<string>()));
            }

            choices[section.Key] = list;
        }

        return choices;
    }

    private IReadOnlyList<ObjectRows> ReadObjects(Source source, string? set)
    {
        var values = new Dictionary<string, Dictionary<string, string>>(StringComparer.Ordinal);
        var order = new List<string>();
        foreach (var path in source.Tables)
        {
            var first = ReferenceEquals(path, source.Tables[0]);
            foreach (var row in ReadTable(Require(path, set), path))
            {
                if (!values.TryGetValue(row.Id, out var merged))
                {
                    if (!first)
                    {
                        continue;
                    }

                    values[row.Id] = merged = new Dictionary<string, string>(StringComparer.Ordinal);
                    order.Add(row.Id);
                }

                foreach (var cell in row.Cells)
                {
                    merged[cell.Key] = cell.Value;
                }
            }
        }

        var wrapped = new Dictionary<string, HashSet<string>>(StringComparer.Ordinal);
        foreach (var pattern in source.Text.Func)
        {
            MergeText(values, wrapped, Expand(pattern, set, true));
        }

        foreach (var pattern in source.Text.Strings)
        {
            MergeText(values, wrapped, Expand(pattern, set, false));
        }

        var objects = new List<ObjectRows>(order.Count);
        foreach (var id in order)
        {
            objects.Add(new ObjectRows(id, values[id], wrapped.TryGetValue(id, out var keys) && keys.Count > 0 ? keys : null));
        }

        return objects;
    }

    private static void MergeText(
        Dictionary<string, Dictionary<string, string>> values,
        Dictionary<string, HashSet<string>> wrapped,
        List<string> files)
    {
        var ids = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
        foreach (var id in values.Keys)
        {
            ids.TryAdd(id, id);
        }

        foreach (var file in files)
        {
            foreach (var section in ReadSections(file, out var wrappedKeys))
            {
                var id = values.ContainsKey(section.Key) ? section.Key : ids.GetValueOrDefault(section.Key);
                if (id == null)
                {
                    continue;
                }

                var merged = values[id];
                if (!wrapped.TryGetValue(id, out var keys))
                {
                    wrapped[id] = keys = new HashSet<string>(StringComparer.Ordinal);
                }

                foreach (var entry in section.Value)
                {
                    merged[entry.Key] = entry.Value;
                    if (wrappedKeys[section.Key].Contains(entry.Key))
                    {
                        keys.Add(entry.Key);
                    }
                    else
                    {
                        keys.Remove(entry.Key);
                    }
                }
            }
        }
    }

    /// <summary>
    /// The files a path or <c>folder/*suffix</c> pattern names, each the set's file when the set replaces it. Locale
    /// strings are never replaced by a set.
    /// </summary>
    private List<string> Expand(string pattern, string? set, bool inSet)
    {
        var slash = pattern.LastIndexOf('/');
        var folder = pattern.Substring(0, slash);
        var name = pattern.Substring(slash + 1);
        var directoryPath = inSet ? folder : LocaleFolder + "/" + folder;
        var useSet = inSet ? set : null;
        if (!name.StartsWith('*'))
        {
            return new List<string> { Require(directoryPath + "/" + name, useSet) };
        }

        var suffix = name.Substring(1);
        var names = new SortedDictionary<string, string>(StringComparer.Ordinal);
        foreach (var root in useSet == null ? new[] { _folder } : new[] { _folder, SetFolder(useSet) })
        {
            var directory = Find(root, directoryPath);
            if (directory == null)
            {
                continue;
            }

            foreach (var file in Directory.EnumerateFiles(directory))
            {
                var fileName = Path.GetFileName(file);
                if (fileName.EndsWith(suffix, StringComparison.OrdinalIgnoreCase))
                {
                    names[fileName.ToLowerInvariant()] = file;
                }
            }
        }

        if (names.Count == 0)
        {
            throw Missing(directoryPath + "/" + name);
        }

        return new List<string>(names.Values);
    }

    private string SetFolder(string set)
    {
        var folder = Find(_folder, "_balance/" + set + ".w3mod");
        if (folder == null)
        {
            throw new DirectoryNotFoundException($"_balance/{set}.w3mod is missing from {_folder}");
        }

        return folder;
    }

    /// <summary>The full path of an export file, the set's when it has one; a missing file is a FileNotFoundException.</summary>
    private string Require(string relative, string? set)
    {
        if (set != null)
        {
            var inSet = Find(SetFolder(set), relative);
            if (inSet != null)
            {
                return inSet;
            }
        }

        return Find(_folder, relative) ?? throw Missing(relative);
    }

    private FileNotFoundException Missing(string relative)
    {
        return new FileNotFoundException($"{relative} is missing from {_folder}", Path.Combine(_folder, relative));
    }

    /// <summary>Finds a path under a folder, matching each segment without regard to case.</summary>
    private static string? Find(string root, string relative)
    {
        var path = root;
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

    private static string ReadText(string path)
    {
        var text = Encoding.UTF8.GetString(File.ReadAllBytes(path));
        return text.StartsWith('\uFEFF') ? text.Substring(1) : text;
    }

    /// <summary>
    /// A <c>[section]</c> and <c>key=value</c> text file. A line that is blank, starts with <c>//</c> or has no
    /// <c>=</c> is ignored; a value wrapped in one pair of quotes loses them and its key is in
    /// <paramref name="wrapped"/> (by section). A section repeated in the file is merged. A key given twice in a section
    /// takes the later value, or with <paramref name="firstWins"/> the first.
    /// </summary>
    private static Dictionary<string, Dictionary<string, string>> ReadSections(
        string path,
        out Dictionary<string, HashSet<string>> wrapped,
        bool firstWins = false)
    {
        var sections = new Dictionary<string, Dictionary<string, string>>(StringComparer.Ordinal);
        wrapped = new Dictionary<string, HashSet<string>>(StringComparer.Ordinal);
        Dictionary<string, string>? section = null;
        HashSet<string>? wrappedKeys = null;
        foreach (var raw in ReadText(path).Split('\n'))
        {
            var line = raw.Trim(' ', '\t', '\r', '\v', '\f');
            if (line.Length == 0 || line.StartsWith("//", StringComparison.Ordinal))
            {
                continue;
            }

            if (line[0] == '[' && line[line.Length - 1] == ']')
            {
                var name = line.Substring(1, line.Length - 2).Trim();
                if (!sections.TryGetValue(name, out section))
                {
                    sections[name] = section = new Dictionary<string, string>(StringComparer.Ordinal);
                    wrapped[name] = new HashSet<string>(StringComparer.Ordinal);
                }

                wrappedKeys = wrapped[name];
                continue;
            }

            var equals = line.IndexOf('=');
            if (section != null && wrappedKeys != null && equals >= 0)
            {
                var key = line.Substring(0, equals).Trim();
                if (firstWins && section.ContainsKey(key))
                {
                    continue;
                }

                var text = line.Substring(equals + 1).Trim();
                var value = Unquote(text);
                section[key] = value;
                if (value.Length != text.Length)
                {
                    wrappedKeys.Add(key);
                }
                else
                {
                    wrappedKeys.Remove(key);
                }
            }
        }

        return sections;
    }

    private static string Unquote(string value)
    {
        if (value.Length >= 2 && value[0] == '"' && value[value.Length - 1] == '"'
            && value.IndexOf('"', 1, value.Length - 2) < 0)
        {
            return value.Substring(1, value.Length - 2);
        }

        return value;
    }

    private sealed record TableRow(string Id, Dictionary<string, string> Cells);

    /// <summary>
    /// An SLK table: the first row names the columns and the first column is the id. Cells are the text of the file,
    /// quoted text unquoted and numbers and booleans as written; empty cells are left out.
    /// </summary>
    private static List<TableRow> ReadTable(string path, string display)
    {
        SylkTable table;
        try
        {
            using var stream = File.OpenRead(path);
            table = new SylkParser(keepText: true).Parse(stream);
        }
        catch (Exception failure) when (failure is FormatException or InvalidDataException or OverflowException)
        {
            throw new FormatException($"{display}: {failure.Message}", failure);
        }

        var header = new SortedDictionary<int, string>();
        for (var x = 0; x <= table.Columns; x++)
        {
            if (table[x, 0] is string name)
            {
                header[x] = name;
            }
        }

        if (header.Count == 0)
        {
            throw new FormatException($"{display} has no header row");
        }

        var idColumn = header.Keys.First();
        var rows = new List<TableRow>();
        for (var y = 1; y <= table.Rows; y++)
        {
            if (table[idColumn, y] is not string { Length: > 0 } id)
            {
                continue;
            }

            var values = new Dictionary<string, string>(StringComparer.Ordinal);
            foreach (var column in header)
            {
                if (table[column.Key, y] is string text)
                {
                    values[column.Value] = text;
                }
            }

            rows.Add(new TableRow(id, values));
        }

        return rows;
    }

    private sealed record Source(string Meta, string[] Tables, TextFiles Text);

    /// <summary>Func and skin files (replaced by a balance set's) and locale strings (never replaced).</summary>
    private sealed record TextFiles(string[] Func, string[] Strings);
}
