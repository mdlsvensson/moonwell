using System.Globalization;

namespace Moonwell.GameData;

/// <summary>One standard object of the game: its identity and its field values under the base tables and each balance set.</summary>
/// <param name="Kind">The kind of object.</param>
/// <param name="Id">The object id (<c>Hpal</c>).</param>
/// <param name="Name">
/// The in-game name: the kind's <c>Name</c> field (an upgrade's at level 1; a buff's tooltip title first), or the
/// empty string. A <c>WESTRING_</c> key, which the game resolves itself, is replaced by the editor's English label
/// (<see cref="GameTables.Label"/>), or by the empty string when the tables have none; <paramref name="Values"/>
/// keeps the key.
/// </param>
/// <param name="Race">The tables' <c>race</c> value as written (<c>human</c>), or the empty string for a kind without one.</param>
/// <param name="Campaign">True for a unit whose <c>campaign</c> column is 1; false for every other kind.</param>
/// <param name="IsHero">True for a unit whose id starts with a capital letter.</param>
/// <param name="Code">An ability's <c>code</c> column, the ability it behaves as; null for every other kind.</param>
/// <param name="Values">
/// The field values under the base tables, as the tables write them (a placeholder <c>-</c> or <c>_</c> of a table
/// cell is no value and is left out), keyed by field id, or <c>id:level</c> for a field of each level (level 1 first;
/// an ability's past its level count too, see <see cref="StandardObjects"/>). For an object the base tables lack, the
/// values under the first set that has it.
/// </param>
/// <param name="Sets">
/// For each balance set the object exists in, the keys whose value differs from <paramref name="Values"/>; the empty
/// string stands for a key the set has no value for. A set the object is missing from has no entry.
/// </param>
/// <param name="MissingFrom">
/// The balance sets the object does not exist under (<c>melee_v0</c>, <c>custom_v0</c>, <c>custom_v1</c>), and
/// <c>base</c> when the base tables lack it.
/// </param>
/// <param name="EditorSuffix">
/// The editor suffix World Editor shows after the name (<c>(Cairne)</c>): the value of the kind's <c>EditorSuffix</c>
/// field (an upgrade's at level 1), trimmed, with a bracket the list's comma cut closed as World Editor does
/// (<c>(ruined, two story)</c> is <c>(ruined)</c>), a <c>WESTRING_</c> key replaced as for <paramref name="Name"/>; the empty
/// string for none, and for a doodad, which has no such field.
/// </param>
/// <param name="Building">
/// True for a unit whose <c>isbldg</c> column is 1 (<c>hbar</c>, the Human Barracks); false for every other kind and
/// for a unit that is not a building (<c>hfoo</c>).
/// </param>
/// <param name="Own">
/// For an ability, the ids of the fields specific to it (<see cref="Fields.SpecificOf"/>), in the fields' order
/// (<c>AHhb</c>, Holy Light, has <c>Hhb1</c>); empty for every other kind. Never null once made.
/// </param>
/// <param name="Modes">
/// The keys of <paramref name="Values"/> whose value differs between the graphics modes (<see cref="StandardObjects.Modes"/>:
/// <c>sd</c> Classic, <c>hd</c> Reforged, <c>de</c> Definitive Edition), read from the same rows as
/// <paramref name="Values"/>, which is the <c>de</c> reading. For each such key, the value in each mode; the empty
/// string stands for a mode the key has no value in (<c>usca</c> of a unit with only <c>modelScale:hd</c> is empty in
/// <c>sd</c>). Never null once made.
/// </param>
public sealed record StandardObject(
    Kind Kind,
    string Id,
    string Name,
    string Race,
    bool Campaign,
    bool IsHero,
    string? Code,
    IReadOnlyDictionary<string, string> Values,
    IReadOnlyDictionary<string, IReadOnlyDictionary<string, string>> Sets,
    IReadOnlyList<string> MissingFrom,
    string EditorSuffix = "",
    bool Building = false,
    IReadOnlyList<string>? Own = null,
    IReadOnlyDictionary<string, IReadOnlyDictionary<string, string>>? Modes = null)
{
    private static readonly IReadOnlyDictionary<string, IReadOnlyDictionary<string, string>> EmptyModes =
        new Dictionary<string, IReadOnlyDictionary<string, string>>();

    /// <summary>The ids of the fields specific to an ability, as the <c>Own</c> parameter above says (<c>Hhb1</c> for <c>AHhb</c>); empty for every other kind. Never null.</summary>
    public IReadOnlyList<string> Own { get; init; } = Own ?? Array.Empty<string>();

    /// <summary>The keys whose value differs between the graphics modes, with the value in each mode (<c>sd</c>, <c>hd</c>, <c>de</c>); empty for none. Never null.</summary>
    public IReadOnlyDictionary<string, IReadOnlyDictionary<string, string>> Modes { get; init; } = Modes ?? EmptyModes;
}

/// <summary>The game's standard objects of a kind, with the value of each field that applies to them.</summary>
/// <remarks>
/// <para>
/// A field applies to a unit by its use flags (a building by <c>useBuilding</c>, a hero by <c>useHero</c>, any other
/// unit by <c>useUnit</c>; a unit is a building when its <c>isbldg</c> is 1); to an ability when it is one of
/// <see cref="Fields.SpecificOf"/> the ability (its id or its <c>code</c> is in the field's <c>Specific</c>), or, for a
/// common field, when its <c>code</c> is not in the metadata's <c>notSpecific</c>; and to every object of another kind.
/// Two fields of one ability that share a column, such as a common <c>Area</c> and a specific one, both get the value.
/// </para>
/// <para>
/// An object exists under a balance set when <see cref="GameTables.Present"/> says so. Its level count is the
/// ability's <c>levels</c> and the upgrade's <c>maxlevel</c> (the field named <c>LevelCount</c>); a per-level field
/// then has a value for each level from 1 to the count. A kind without a level count (a doodad's tinting colours)
/// has a value for each entry of its comma list.
/// </para>
/// <para>
/// An ability's table holds columns past its level count too, which the game reads when a map raises the count: on
/// 3.0.1 a Storm Bolt of six levels deals 325 at levels 4 to 6, its fourth column, and costs 75 mana there (game-facts,
/// seen 2026-10-19; the patch notes of 3.0.1: "Ability data beyond the 4th level now properly retain their default
/// values"). So a field held in a column of each level also has the values of the columns past the count, up to the
/// table's last; a level past the 4th with no value of its own takes the level before's.
/// </para>
/// <para>
/// Keys are looked up as the tables spell them and, failing that, without regard to case when one key matches
/// (<c>Ubertip</c> is <c>UberTip</c> in the text files). A per-level text value held in the text files is a comma
/// list, level 1 first, unless it was wrapped in quotes (<see cref="ObjectRows.Wrapped"/>), when it is one entry; a
/// list with fewer entries than levels has values for the levels it names only.
/// </para>
/// </remarks>
public static class StandardObjects
{
    private const string BaseTables = "base";
    private const string LevelCountName = "LevelCount";
    private const string NameName = "Name";
    private const string TooltipName = "Tooltip";
    private const string SuffixColumn = "EditorSuffix";
    private const string KeyPrefix = "WESTRING_";
    private const string DefinitiveMode = "de";

    /// <summary>The graphics modes, by the names <see cref="StandardObject.Modes"/> uses: <c>sd</c> Classic, <c>hd</c> Reforged, <c>de</c> Definitive Edition. <see cref="StandardObject.Values"/> is the <c>de</c> reading.</summary>
    public static readonly IReadOnlyList<string> Modes = ["sd", "hd", "de"];

    /// <summary>The standard objects of <paramref name="kind"/> ordered by id (ordinal), with the values of <paramref name="fields"/>.</summary>
    public static IReadOnlyList<StandardObject> Of(GameTables tables, Kind kind, IReadOnlyList<Field> fields)
    {
        var meta = new Dictionary<string, MetaRow>(StringComparer.Ordinal);
        foreach (var row in tables.Meta(kind))
        {
            meta[row.Id] = row;
        }

        var plans = new List<FieldPlan>();
        foreach (var field in fields)
        {
            plans.Add(new FieldPlan(field, meta[field.Id]));
        }

        var baseRows = Index(tables.Objects(kind, null));
        var setNames = tables.Sets;
        var setRows = new Dictionary<string, Dictionary<string, ObjectRows>>(StringComparer.Ordinal);
        var present = new Dictionary<string, IReadOnlySet<string>>(StringComparer.Ordinal);
        var ids = new SortedSet<string>(baseRows.Keys, StringComparer.Ordinal);
        foreach (var set in setNames)
        {
            setRows[set] = Index(tables.Objects(kind, set));
            present[set] = tables.Present(kind, set);
            ids.UnionWith(present[set]);
        }

        if (kind == Kind.Ability)
        {
            var keys = new HashSet<string>(StringComparer.Ordinal);
            foreach (var rows in new[] { baseRows }.Concat(setRows.Values))
            {
                foreach (var row in rows.Values)
                {
                    keys.UnionWith(row.Values.Keys);
                }
            }

            foreach (var plan in plans)
            {
                plan.FindLastLevel(keys);
            }
        }

        var evaluator = new Evaluator(kind, plans);

        var objects = new List<StandardObject>(ids.Count);
        foreach (var id in ids)
        {
            objects.Add(Build(tables, kind, id, evaluator, baseRows, setNames, setRows, present));
        }

        return objects;
    }

    private static Dictionary<string, ObjectRows> Index(IReadOnlyList<ObjectRows> rows)
    {
        var index = new Dictionary<string, ObjectRows>(StringComparer.Ordinal);
        foreach (var row in rows)
        {
            index[row.Id] = row;
        }

        return index;
    }

    private static StandardObject Build(
        GameTables tables,
        Kind kind,
        string id,
        Evaluator evaluator,
        Dictionary<string, ObjectRows> baseRows,
        IReadOnlyList<string> setNames,
        Dictionary<string, Dictionary<string, ObjectRows>> setRows,
        Dictionary<string, IReadOnlySet<string>> present)
    {
        var missing = new List<string>();
        var existing = new List<string>();
        foreach (var set in setNames)
        {
            (present[set].Contains(id) ? existing : missing).Add(set);
        }

        ObjectRows primary;
        if (baseRows.TryGetValue(id, out var baseRow))
        {
            primary = baseRow;
        }
        else
        {
            primary = setRows[existing[0]][id];
            missing.Insert(0, BaseTables);
        }

        var row = new Row(primary, DefinitiveMode);
        var values = evaluator.Values(id, row);
        var sets = new Dictionary<string, IReadOnlyDictionary<string, string>>(StringComparer.Ordinal);
        foreach (var set in existing)
        {
            sets[set] = Diff(values, evaluator.Values(id, new Row(setRows[set][id], DefinitiveMode)));
        }

        // Only the skin files key values by mode, so most objects need no second reading.
        var modes = new Dictionary<string, IReadOnlyDictionary<string, string>>(StringComparer.Ordinal);
        if (row.HasModes)
        {
            var byMode = Modes.Select(mode => mode == DefinitiveMode ? values : evaluator.Values(id, row.In(mode))).ToArray();
            foreach (var key in byMode.SelectMany(read => read.Keys).Distinct(StringComparer.Ordinal))
            {
                var perMode = new Dictionary<string, string>(StringComparer.Ordinal);
                for (var i = 0; i < Modes.Count; i++)
                {
                    perMode[Modes[i]] = byMode[i].GetValueOrDefault(key, "");
                }

                if (perMode.Values.Distinct(StringComparer.Ordinal).Count() > 1)
                {
                    modes[key] = perMode;
                }
            }
        }

        return new StandardObject(
            kind,
            id,
            Resolved(tables, evaluator.NameOf(values)),
            Clean(row.Get("race")) ?? "",
            kind == Kind.Unit && Clean(row.Get("campaign")) == "1",
            IsHero(kind, id),
            kind == Kind.Ability ? Clean(row.Get("code")) : null,
            values,
            sets,
            missing,
            Resolved(tables, evaluator.SuffixOf(values)).Trim(),
            evaluator.BuildingOf(row),
            evaluator.OwnOf(id, kind == Kind.Ability ? Clean(row.Get("code")) : null),
            modes);
    }

    /// <summary>Whether a field applies to an object: by use flags for a unit, by its specific and <c>notSpecific</c> codes for an ability, always for any other kind.</summary>
    /// <param name="field">A field of the object's kind.</param>
    /// <param name="obj">The object, as <see cref="Of"/> built it.</param>
    /// <returns>True when the object has a value for the field to hold, e.g. <c>ustr</c> (Strength) applies to a hero but not to a footman.</returns>
    public static bool Applies(Field field, StandardObject obj)
    {
        return Applies(field, obj.Kind, obj.Code, obj.IsHero, obj.Building, obj.Own);
    }

    /// <summary>The rule of <see cref="Applies(Field, StandardObject)"/>, written once for it and for the generator's reading of the tables.</summary>
    private static bool Applies(Field field, Kind kind, string? code, bool hero, bool building, IEnumerable<string>? own)
    {
        switch (kind)
        {
            case Kind.Unit:
                return building ? field.ForBuilding : hero ? field.ForHero : field.ForUnit;
            case Kind.Ability:
                return field.Specific.Count > 0 ? own != null && own.Contains(field.Id) : !field.NotSpecific.Contains(code ?? "");
            default:
                return true;
        }
    }

    /// <summary>The label of a <c>WESTRING_</c> key; the empty string for a key the tables have no label for; any other name as it is.</summary>
    private static string Resolved(GameTables tables, string name)
    {
        var label = tables.Label(name);
        return string.Equals(label, name, StringComparison.Ordinal) && name.StartsWith(KeyPrefix, StringComparison.Ordinal) ? "" : label;
    }

    private static bool IsHero(Kind kind, string id)
    {
        return kind == Kind.Unit && id.Length > 0 && id[0] >= 'A' && id[0] <= 'Z';
    }

    private static Dictionary<string, string> Diff(Dictionary<string, string> baseValues, Dictionary<string, string> setValues)
    {
        var diff = new Dictionary<string, string>(StringComparer.Ordinal);
        foreach (var pair in setValues)
        {
            if (!baseValues.TryGetValue(pair.Key, out var original) || !string.Equals(original, pair.Value, StringComparison.Ordinal))
            {
                diff[pair.Key] = pair.Value;
            }
        }

        foreach (var pair in baseValues)
        {
            if (!setValues.ContainsKey(pair.Key))
            {
                diff[pair.Key] = "";
            }
        }

        return diff;
    }

    /// <summary>A cell's value, trimmed: null when empty or a table's placeholder <c>-</c> or <c>_</c>.</summary>
    private static string? Clean(string? cell)
    {
        var value = cell?.Trim();
        return string.IsNullOrEmpty(value) || value == "-" || value == "_" ? null : value;
    }

    /// <summary>
    /// The entries of a comma list; an empty entry is null. An entry that starts with a quote ends at the next
    /// quote, so it may hold commas and loses its quotes.
    /// </summary>
    private static List<string?> Entries(string text)
    {
        var entries = new List<string?>();
        var i = 0;
        while (true)
        {
            while (i < text.Length && (text[i] == ' ' || text[i] == '\t'))
            {
                i++;
            }

            string entry;
            var close = i < text.Length && text[i] == '"' ? text.IndexOf('"', i + 1) : -1;
            if (close >= 0)
            {
                entry = text.Substring(i + 1, close - i - 1);
                var comma = text.IndexOf(',', close + 1);
                i = comma < 0 ? text.Length : comma;
            }
            else
            {
                var comma = text.IndexOf(',', i);
                var end = comma < 0 ? text.Length : comma;
                entry = text.Substring(i, end - i);
                i = end;
            }

            entries.Add(Clean(entry));
            if (i >= text.Length)
            {
                return entries;
            }

            i++;
        }
    }

    /// <summary>
    /// The keys of one object's merged rows, looked up as spelled and then without regard to case. The skin files key some
    /// values by graphics mode (<c>modelScale:sd</c>, <c>modelScale:de</c>, <c>modelScale:hd</c>; the modes of 3.0.0's patch
    /// notes: Classic, Definitive Edition and Reforged). The standard value is the Definitive Edition graphics' value, which
    /// is what the game read in graphics mode 2 on 3.0.1 (game-facts, seen 2026-10-22): the key's <c>:de</c> value, else its
    /// <c>:hd</c> value, else the key itself, and never its <c>:sd</c> value (a footman's scale 0.8 with <c>:hd</c> 0.95, a
    /// peasant's 0.85 beside its <c>modelScale=1</c>, an assassin's 1.2 with <c>:hd</c> 1.2 and <c>:sd</c> 1.3). In the
    /// other modes the game reads that mode's values: Classic the key's <c>:sd</c> value, else the key itself; Reforged
    /// its <c>:hd</c> value, else the key itself (<see cref="In"/>).
    /// </summary>
    private sealed class Row
    {
        // A blank :de value is read as the value, so the key has none: the tables' only one, Aent's Casterart:de, leaves
        // Entangle Gold Mine without its caster art. Not checked in the game. A blank :hd value, with no :de one, is read
        // the same way.
        private const string GameMode = ":de";
        private const string FallbackMode = ":hd";
        private const string ClassicMode = ":sd";

        private readonly ObjectRows _rows;
        private readonly string _mode;
        private Dictionary<string, string?>? _folded;
        private bool? _hasModes;

        /// <summary>The rows as read in a graphics mode (<see cref="StandardObjects.Modes"/>).</summary>
        public Row(ObjectRows rows, string mode)
        {
            _rows = rows;
            _mode = mode;
        }

        private Row(Row from, string mode)
        {
            _rows = from._rows;
            _mode = mode;
            _folded = from.Folded();
        }

        /// <summary>Whether the rows hold a key with a mode suffix (<c>modelScale:hd</c>), so that the modes may read different values.</summary>
        public bool HasModes => _hasModes ??= _rows.Values.Keys.Any(HasModeSuffix);

        /// <summary>The same rows read in another mode.</summary>
        public Row In(string mode)
        {
            return new Row(this, mode);
        }

        /// <summary>The value of the key, or null when no key matches or several keys match without regard to case.</summary>
        public string? Get(string key)
        {
            var spelled = Spelling(key);
            return spelled == null ? null : _rows.Values[spelled];
        }

        /// <summary>
        /// The entries of the key's comma list, null when there is no value. A value that was wrapped in quotes in
        /// the text file is one entry, with its commas.
        /// </summary>
        public List<string?>? Entries(string key)
        {
            var spelled = Spelling(key);
            if (spelled == null)
            {
                return null;
            }

            var value = _rows.Values[spelled];
            if (_rows.Wrapped != null && _rows.Wrapped.Contains(spelled))
            {
                return new List<string?> { Clean(value) };
            }

            return StandardObjects.Entries(value);
        }

        private string? Spelling(string key)
        {
            return _mode switch
            {
                "sd" => Spelled(key + ClassicMode) ?? Spelled(key),
                "hd" => Spelled(key + FallbackMode) ?? Spelled(key),
                _ => Spelled(key + GameMode) ?? Spelled(key + FallbackMode) ?? Spelled(key),
            };
        }

        private static bool HasModeSuffix(string key)
        {
            return key.EndsWith(GameMode, StringComparison.OrdinalIgnoreCase)
                || key.EndsWith(FallbackMode, StringComparison.OrdinalIgnoreCase)
                || key.EndsWith(ClassicMode, StringComparison.OrdinalIgnoreCase);
        }

        private string? Spelled(string key)
        {
            if (_rows.Values.ContainsKey(key))
            {
                return key;
            }

            return Folded().GetValueOrDefault(key);
        }

        /// <summary>The keys without regard to case, each to its spelling, or to null when several keys match.</summary>
        private Dictionary<string, string?> Folded()
        {
            if (_folded == null)
            {
                _folded = new Dictionary<string, string?>(StringComparer.OrdinalIgnoreCase);
                foreach (var pair in _rows.Values)
                {
                    _folded[pair.Key] = _folded.ContainsKey(pair.Key) ? null : pair.Key;
                }
            }

            return _folded;
        }
    }

    /// <summary>How one field's value is found in an object's rows, and which objects it applies to.</summary>
    private sealed class FieldPlan
    {
        private readonly string _column;
        private readonly bool _perLevelColumn;
        private readonly int _element;
        private readonly bool _list;
        private readonly bool _appended;
        private readonly bool _suffix;

        // The last level whose column the tables hold, for an ability field held in a column of each level; 0 otherwise.
        private int _lastLevel;

        // The last level whose value has no carry from the level before: the game keeps level 4's data for the levels past it.
        private const int LastLevelOfItsOwn = 4;

        public FieldPlan(Field field, MetaRow meta)
        {
            Field = field;
            _appended = Cell(meta, "appendIndex") == "1";
            _element = -1;
            _column = field.Column;
            if (_column.EndsWith(']'))
            {
                var open = _column.LastIndexOf('[');
                _element = int.Parse(_column.AsSpan(open + 1, _column.Length - open - 2), NumberStyles.None, CultureInfo.InvariantCulture);
                _column = _column.Substring(0, open);
            }

            _perLevelColumn = _column.Contains("{level}", StringComparison.Ordinal);
            _list = field.PerLevel && !_perLevelColumn;
            _suffix = _column == SuffixColumn;
            if (_element >= 0 && field.PerLevel)
            {
                throw new InvalidOperationException($"{field.Kind} field {field.Id}: a per-level field that is one element of a list is not supported");
            }
        }

        public Field Field { get; }

        /// <summary>Finds the last level whose column <paramref name="keys"/> (every key of the kind's rows) holds: <c>DataA6</c> gives 6.</summary>
        public void FindLastLevel(IReadOnlySet<string> keys)
        {
            if (!_perLevelColumn)
            {
                return;
            }

            while (keys.Contains(KeyOf(_lastLevel + 1)))
            {
                _lastLevel++;
            }
        }

        /// <summary>Whether the field applies to an object.</summary>
        /// <param name="kind">The object's kind.</param>
        /// <param name="code">For an ability, its code, which the field's <c>notSpecific</c> list may exclude.</param>
        /// <param name="hero">For a unit, whether it is a hero.</param>
        /// <param name="building">For a unit, whether it is a building.</param>
        /// <param name="own">For an ability, the ids of the fields specific to it (<see cref="Fields.SpecificOf"/>).</param>
        public bool Applies(Kind kind, string? code, bool hero, bool building, IReadOnlySet<string>? own)
        {
            return StandardObjects.Applies(Field, kind, code, hero, building, own);
        }

        /// <summary>Adds the field's values: one, or one for each level up to <paramref name="levels"/> (null: no level count).</summary>
        public void Read(Row row, int? levels, Dictionary<string, string> values)
        {
            if (_perLevelColumn)
            {
                string? previous = null;
                for (var level = 1; level <= (levels ?? 0); level++)
                {
                    previous = Clean(row.Get(KeyOf(level)));
                    Add(values, level, previous);
                }

                // Past the level count, up to the table's last column: a level past the 4th with no value takes the level before's.
                for (var level = (levels ?? _lastLevel) + 1; level <= _lastLevel; level++)
                {
                    previous = Clean(row.Get(KeyOf(level))) ?? (level > LastLevelOfItsOwn ? previous : null);
                    Add(values, level, previous);
                }
            }
            else if (_list && _appended)
            {
                for (var level = 1; level <= (levels ?? 0); level++)
                {
                    var key = level == 1 ? _column : _column + (level - 1).ToString(CultureInfo.InvariantCulture);
                    Add(values, level, Clean(row.Get(key)));
                }
            }
            else if (_list)
            {
                var entries = row.Entries(_column);
                for (var i = 0; entries != null && i < entries.Count && (levels == null || i < levels); i++)
                {
                    Add(values, i + 1, entries[i]);
                }
            }
            else if (_element >= 0)
            {
                var entries = row.Entries(_column);
                if (entries != null && _element < entries.Count)
                {
                    Add(values, 0, entries[_element]);
                }
            }
            else
            {
                Add(values, 0, Clean(row.Get(_column)));
            }
        }

        private string KeyOf(int level)
        {
            return _column.Replace("{level}", level.ToString(CultureInfo.InvariantCulture), StringComparison.Ordinal);
        }

        private void Add(Dictionary<string, string> values, int level, string? value)
        {
            // World Editor closes a suffix that the list's comma cut inside its brackets: (ruined, two story) is (ruined).
            if (_suffix && value != null && value.StartsWith('(') && !value.Contains(')', StringComparison.Ordinal))
            {
                value += ")";
            }

            if (value != null)
            {
                values[level == 0 ? Field.Id : Field.Id + ":" + level.ToString(CultureInfo.InvariantCulture)] = value;
            }
        }

        private static string Cell(MetaRow meta, string column)
        {
            return meta.Columns.TryGetValue(column, out var value) ? value.Trim() : "";
        }
    }

    /// <summary>The values of every field that applies to an object of one kind.</summary>
    private sealed class Evaluator
    {
        private readonly Kind _kind;
        private readonly List<FieldPlan> _plans;
        private readonly List<Field> _fields;
        private readonly FieldPlan? _levels;
        private readonly List<string> _nameIds = new();
        private readonly string? _suffixId;

        public Evaluator(Kind kind, List<FieldPlan> plans)
        {
            _kind = kind;
            _plans = plans;
            _fields = plans.Select(plan => plan.Field).ToList();
            string? name = null;
            string? tooltip = null;
            foreach (var plan in plans)
            {
                if (plan.Field.Name == LevelCountName)
                {
                    _levels = plan;
                }
                else if (plan.Field.Name == NameName)
                {
                    name = plan.Field.Id;
                }
                else if (plan.Field.Name == TooltipName)
                {
                    tooltip = plan.Field.Id;
                }

                if (plan.Field.Column == SuffixColumn || plan.Field.Column.StartsWith(SuffixColumn + "[", StringComparison.Ordinal))
                {
                    _suffixId = plan.Field.Id;
                }
            }

            // A buff's name in the game is the tooltip title; its Name field is the editor's.
            if (kind == Kind.Buff && tooltip != null)
            {
                _nameIds.Add(tooltip);
            }

            if (name != null)
            {
                _nameIds.Add(name);
            }
        }

        public Dictionary<string, string> Values(string id, Row row)
        {
            var code = _kind == Kind.Ability ? Clean(row.Get("code")) : null;
            var hero = IsHero(_kind, id);
            var building = BuildingOf(row);
            var levels = LevelCount(row);
            var own = _kind == Kind.Ability ? new HashSet<string>(OwnOf(id, code), StringComparer.Ordinal) : null;
            var values = new Dictionary<string, string>(StringComparer.Ordinal);
            foreach (var plan in _plans)
            {
                if (plan.Applies(_kind, code, hero, building, own))
                {
                    plan.Read(row, levels, values);
                }
            }

            return values;
        }

        /// <summary>Whether the object is a unit whose <c>isbldg</c> is 1.</summary>
        public bool BuildingOf(Row row)
        {
            return _kind == Kind.Unit && Clean(row.Get("isbldg")) == "1";
        }

        /// <summary>The ids of the ability's own fields (<see cref="Fields.SpecificOf"/>); empty for any other kind.</summary>
        public List<string> OwnOf(string id, string? code)
        {
            return _kind == Kind.Ability ? Fields.SpecificOf(_fields, id, code).Select(field => field.Id).ToList() : new List<string>();
        }

        /// <summary>The name in <paramref name="values"/>: the <c>Name</c> field's (an upgrade's at level 1; a buff's tooltip first).</summary>
        public string NameOf(Dictionary<string, string> values)
        {
            foreach (var id in _nameIds)
            {
                if (values.TryGetValue(id, out var name) || values.TryGetValue(id + ":1", out name))
                {
                    return name;
                }
            }

            return "";
        }

        /// <summary>The editor suffix in <paramref name="values"/> (an upgrade's at level 1); the empty string for none.</summary>
        public string SuffixOf(Dictionary<string, string> values)
        {
            if (_suffixId != null && (values.TryGetValue(_suffixId, out var suffix) || values.TryGetValue(_suffixId + ":1", out suffix)))
            {
                return suffix;
            }

            return "";
        }

        private int? LevelCount(Row row)
        {
            if (_levels == null)
            {
                return null;
            }

            var cell = new Dictionary<string, string>(StringComparer.Ordinal);
            _levels.Read(row, null, cell);
            if (!cell.TryGetValue(_levels.Field.Id, out var text))
            {
                return 0;
            }

            if (!int.TryParse(text, NumberStyles.None, CultureInfo.InvariantCulture, out var count))
            {
                throw new FormatException($"{_kind} {_levels.Field.Id}: the level count '{text}' is not a whole number");
            }

            return count;
        }
    }
}
