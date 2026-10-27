using Moonwell.Generator.Objects;

namespace Moonwell.Generator.Tests;

public class FieldsTests
{
    private static ExportTables MiniExport()
    {
        return ExportTables.Read(Path.GetFullPath(
            Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Fixtures", "MiniExport", "war3.w3mod")));
    }

    private static Field Find(IReadOnlyList<Field> fields, string id)
    {
        return fields.Single(f => f.Id == id);
    }

    [Fact]
    public void an_abilitys_fields_are_those_listed_by_its_id_or_its_code_and_one_listed_by_id_wins_its_column()
    {
        static Field Specific(string id, string column, params string[] specific)
        {
            return new Field(Kind.Ability, id, id, FieldType.Int, null, true, 1, false, specific, column);
        }

        var fields = new[]
        {
            Specific("Car1", "DataA{level}", "Sch2", "Sch3"),
            Specific("Lsh1", "DataA{level}", "Alsh"),
            Specific("Idps", "DataA{level}", "AIls"),
            Specific("Lsh2", "DataB{level}", "Alsh"),
        };

        Assert.Equal(new[] { "Car1" }, Fields.SpecificOf(fields, "Sch3", "Amtc").Select(field => field.Id));
        Assert.Equal(new[] { "Idps", "Lsh2" }, Fields.SpecificOf(fields, "AIls", "Alsh").Select(field => field.Id));
        Assert.Equal(new[] { "Lsh1", "Lsh2" }, Fields.SpecificOf(fields, "Alsh", "Alsh").Select(field => field.Id));
        Assert.Empty(Fields.SpecificOf(fields, "Aabc", null));
    }

    [Theory]
    [InlineData(Kind.Unit, "umvs", "Speed")]
    [InlineData(Kind.Unit, "udef", "Armor")]
    [InlineData(Kind.Unit, "uabi", "Abilities")]
    [InlineData(Kind.Ability, "acdn", "Cooldown")]
    [InlineData(Kind.Ability, "amcs", "ManaCost")]
    [InlineData(Kind.Ability, "alev", "LevelCount")]
    [InlineData(Kind.Upgrade, "glvl", "LevelCount")]
    [InlineData(Kind.Ability, "anam", "Name")]
    [InlineData(Kind.Ability, "abuf", "Buffs")]
    [InlineData(Kind.Ability, "atar", "TargetsAllowed")]
    [InlineData(Kind.Ability, "Det1", "Detects")]
    [InlineData(Kind.Ability, "Fla1", "Detects")]
    [InlineData(Kind.Ability, "Ifa1", "Detects")]
    [InlineData(Kind.Ability, "Ndt3", "Detects")]
    [InlineData(Kind.Ability, "Nsp3", "Detects")]
    [InlineData(Kind.Ability, "Ofs1", "Detects")]
    [InlineData(Kind.Ability, "Hca4", "StackingType")]
    [InlineData(Kind.Item, "iabi", "Abilities")]
    public void hand_picked_names(Kind kind, string id, string name)
    {
        Assert.Equal(name, FieldNames.HandPicked(kind)[id]);
        Assert.Equal(name, Find(Fields.Of(MiniExport(), kind), id).Name);
    }

    [Theory]
    [InlineData(Kind.Unit, "uhpm", "MaxLife")]
    [InlineData(Kind.Unit, "umpm", "MaxMana")]
    [InlineData(Kind.Unit, "ulev", "Level")]
    [InlineData(Kind.Unit, "usca", "Scale")]
    [InlineData(Kind.Unit, "unam", "Name")]
    [InlineData(Kind.Unit, "ustr", "Strength")]
    [InlineData(Kind.Unit, "uagi", "Agility")]
    [InlineData(Kind.Unit, "uint", "Intelligence")]
    [InlineData(Kind.Item, "iuse", "Charges")]
    public void a_shared_field_takes_its_live_members_name_though_the_runtime_class_has_it(Kind kind, string id, string name)
    {
        var runtime = RuntimeMembers.Read(Path.Combine(AppContext.BaseDirectory, "Moonwell.dll")).Of(kind);

        Assert.Contains(name, runtime);
        Assert.Equal(name, SharedFields.MemberOf(kind, id));
        Assert.False(FieldNames.HandPicked(kind).ContainsKey(id));
        Assert.Equal(name, Find(Fields.Of(MiniExport(), kind, runtime), id).Name);
    }

    [Fact]
    public void the_shared_fields_are_the_specs_table()
    {
        Assert.Equal(
            new[] { "uagi Agility", "uhpm MaxLife", "uint Intelligence", "ulev Level", "umpi Mana", "umpm MaxMana", "umvh FlyHeight", "unam Name", "usca Scale", "ustr Strength" },
            SharedFields.Of(Kind.Unit).OrderBy(p => p.Key, StringComparer.Ordinal).Select(p => p.Key + " " + p.Value));
        Assert.Equal(new[] { "iuse Charges" }, SharedFields.Of(Kind.Item).Select(p => p.Key + " " + p.Value));
        Assert.Empty(SharedFields.Of(Kind.Destructable));
        Assert.Null(SharedFields.MemberOf(Kind.Item, "unam"));
    }

    [Theory]
    [InlineData(Kind.Unit, "utub", "TooltipExtended")]
    [InlineData(Kind.Item, "utub", "TooltipExtended")]
    [InlineData(Kind.Ability, "aub1", "TooltipExtended")]
    [InlineData(Kind.Ability, "aret", "TooltipLearn")]
    [InlineData(Kind.Ability, "arut", "TooltipLearnExtended")]
    [InlineData(Kind.Buff, "fube", "TooltipExtended")]
    [InlineData(Kind.Upgrade, "gub1", "TooltipExtended")]
    public void tooltip_fields_follow_world_editor_order(Kind kind, string id, string name)
    {
        Assert.Equal(name, FieldNames.HandPicked(kind)[id]);
    }

    [Fact]
    public void every_listed_id_exists()
    {
        var tables = MiniExport();

        foreach (var kind in Enum.GetValues<Kind>())
        {
            var ids = Fields.Of(tables, kind).Select(f => f.Id).ToHashSet();
            foreach (var id in FieldNames.HandPicked(kind).Keys)
            {
                Assert.True(ids.Contains(id), $"{kind} {id} is listed in field-names.txt but is not a field of the kind");
            }
        }
    }

    private static readonly System.Text.RegularExpressions.Regex ClassOrProperty = new(
        @"public (?:sealed |abstract )?class (?<class>\w+)|public (?<type>[^\r\n(]+?) (?<name>\w+)\r?\n\s*\{\r?\n\s*get \{ return Fields\.\w+(?:<[^>]+>)?\(this, ""(?<id>\w+)""\)",
        System.Text.RegularExpressions.RegexOptions.Compiled);

    /// <summary>Every generated field property: its class, name and field id.</summary>
    private static List<(string Class, string Name, string Id)> GeneratedProperties()
    {
        var generated = "";
        for (var folder = new DirectoryInfo(AppContext.BaseDirectory); folder != null; folder = folder.Parent)
        {
            var candidate = Path.Combine(folder.FullName, "src", "Moonwell.Objects", "Generated");
            if (Directory.Exists(candidate))
            {
                generated = candidate;
                break;
            }
        }

        Assert.NotEqual("", generated);
        var properties = new List<(string, string, string)>();
        foreach (var file in Directory.EnumerateFiles(generated, "*.cs", SearchOption.AllDirectories))
        {
            var current = "";
            foreach (System.Text.RegularExpressions.Match match in ClassOrProperty.Matches(File.ReadAllText(file)))
            {
                if (match.Groups["class"].Success)
                {
                    current = match.Groups["class"].Value;
                }
                else
                {
                    properties.Add((current, match.Groups["name"].Value, match.Groups["id"].Value));
                }
            }
        }

        return properties;
    }

    [Fact]
    public void the_generated_properties_are_found()
    {
        // The two tests below read the generated sources; they prove nothing if the pattern finds nothing.
        var properties = GeneratedProperties();

        Assert.True(properties.Count > 5000);
        Assert.Contains(("StormBolt", "Damage", "Htb1"), properties);
    }

    [Theory]
    [InlineData("Inferno", "InfernalDuration", "Uin2")]
    [InlineData("Taunt", "PreferHostiles", "Tau2")]
    [InlineData("Taunt", "PreferFriendlies", "Tau3")]
    [InlineData("WindWalk", "BackstabDamage", "Owk3")]
    [InlineData("WindWalk", "Backstab", "Owk4")]
    [InlineData("HealingModifier", "HealingPercent", "hem1")]
    public void a_field_that_took_its_id_has_a_name_of_its_own(string className, string name, string id)
    {
        Assert.Contains((className, name, id), GeneratedProperties());
        Assert.Equal(name, FieldNames.HandPicked(Kind.Ability)[id]);
    }

    [Fact]
    public void no_generated_property_name_ends_in_its_own_field_id()
    {
        // A name that ends in its id (DurationUin2) is the generator's way out of two fields with one label; the game's
        // data for such a field gets a name of its own in field-names.txt instead.
        var ending = GeneratedProperties()
            .Where(p => p.Name.EndsWith(p.Id, StringComparison.OrdinalIgnoreCase))
            .Select(p => $"{p.Class}.{p.Name} ({p.Id})")
            .Distinct()
            .OrderBy(p => p, StringComparer.Ordinal)
            .ToList();

        Assert.Empty(ending);
    }

    [Fact]
    public void types_from_metadata()
    {
        var tables = MiniExport();

        var hitPoints = Find(Fields.Of(tables, Kind.Unit), "uhpm");
        Assert.Equal(FieldType.Int, hitPoints.Type);
        Assert.Null(hitPoints.ListOf);
        Assert.False(hitPoints.Skin);

        var abilities = Find(Fields.Of(tables, Kind.Unit), "uabi");
        Assert.Equal(FieldType.ObjectList, abilities.Type);
        Assert.Equal("Ability", abilities.ListOf);

        var name = Find(Fields.Of(tables, Kind.Unit), "unam");
        Assert.Equal(FieldType.String, name.Type);
        Assert.True(name.Skin);

        Assert.Equal(FieldType.Float, Find(Fields.Of(tables, Kind.Unit), "uhpr").Type);
        Assert.Equal(FieldType.Bool, Find(Fields.Of(tables, Kind.Ability), "aher").Type);
        Assert.Equal("object", Find(Fields.Of(tables, Kind.Unit), "ureq").ListOf);
        Assert.Equal("Unit", Find(Fields.Of(tables, Kind.Unit), "ubui").ListOf);
    }

    [Fact]
    public void store_from_metadata_type()
    {
        var tables = MiniExport();

        Assert.Equal("int", Find(Fields.Of(tables, Kind.Unit), "uhpm").Store);
        Assert.Equal("string", Find(Fields.Of(tables, Kind.Unit), "unam").Store);
        Assert.Equal("string", Find(Fields.Of(tables, Kind.Unit), "uabi").Store);
        Assert.Equal("unreal", Find(Fields.Of(tables, Kind.Unit), "uhpr").Store);
        Assert.Equal("int", Find(Fields.Of(tables, Kind.Ability), "aher").Store);
        Assert.Equal("unreal", Find(Fields.Of(tables, Kind.Ability), "Htb1").Store);
    }

    [Fact]
    public void per_level_and_pointer()
    {
        var field = Find(Fields.Of(MiniExport(), Kind.Ability), "Htb1");

        Assert.True(field.PerLevel);
        Assert.Equal(1, field.DataPointer);
        Assert.Contains("AHtb", field.Specific);
        Assert.Equal(FieldType.Float, field.Type);
        Assert.Equal("DataA{level}", field.Column);
        Assert.False(Find(Fields.Of(MiniExport(), Kind.Ability), "alev").PerLevel);
        Assert.Empty(Find(Fields.Of(MiniExport(), Kind.Ability), "alev").Specific);
        Assert.Equal(0, Find(Fields.Of(MiniExport(), Kind.Ability), "acdn").DataPointer);
    }

    [Fact]
    public void columns_find_the_values()
    {
        var tables = MiniExport();

        Assert.Equal("HP", Find(Fields.Of(tables, Kind.Unit), "uhpm").Column);
        Assert.Equal("Cool{level}", Find(Fields.Of(tables, Kind.Ability), "acdn").Column);
        Assert.Equal("Tip", Find(Fields.Of(tables, Kind.Ability), "atp1").Column);
        Assert.True(Find(Fields.Of(tables, Kind.Ability), "atp1").PerLevel);
        Assert.Equal("Buttonpos[0]", Find(Fields.Of(tables, Kind.Unit), "ubpx").Column);
        Assert.Equal("Buttonpos[1]", Find(Fields.Of(tables, Kind.Unit), "ubpy").Column);
        Assert.Equal("Buttonpos[1]", Find(Fields.Of(tables, Kind.Ability), "abpy").Column);
        Assert.Equal("Name", Find(Fields.Of(tables, Kind.Unit), "unam").Column);
        Assert.Equal("Requires", Find(Fields.Of(tables, Kind.Upgrade), "greq").Column);
        Assert.Equal("Name", Find(Fields.Of(tables, Kind.Destructable), "bnam").Column);
    }

    [Fact]
    public void units_and_items_take_their_own_fields()
    {
        var tables = MiniExport();
        var units = Fields.Of(tables, Kind.Unit).Select(f => f.Id).ToList();
        var items = Fields.Of(tables, Kind.Item).Select(f => f.Id).ToList();

        Assert.Contains("uabi", units);
        Assert.DoesNotContain("uabi", items);
        Assert.Contains("iabi", items);
        Assert.DoesNotContain("iabi", units);
        Assert.Contains("unam", units);
        Assert.Contains("unam", items);
        Assert.All(Fields.Of(tables, Kind.Item), f => Assert.Equal(Kind.Item, f.Kind));
        Assert.Equal(units.OrderBy(i => i, StringComparer.Ordinal), units);
    }

    [Fact]
    public void name_from_a_metadata_label()
    {
        var tables = MiniExport();

        Assert.Equal("ButtonPositionX", Find(Fields.Of(tables, Kind.Unit), "ubpx").Name);
        Assert.Equal("RequirementsTier2", Find(Fields.Of(tables, Kind.Unit), "urq1").Name);
    }

    [Theory]
    [InlineData("Hit Points Maximum (Base)", "HitPointsMaximumBase")]
    [InlineData("Area of Effect", "AreaOfEffect")]
    [InlineData("Caster's Level", "CastersLevel")]
    [InlineData("Damage/Healing - Bonus", "DamageHealingBonus")]
    [InlineData("1st Attack", "_1stAttack")]
    [InlineData("Attack 1 - Range", "Attack1Range")]
    [InlineData("  spaced   out  ", "SpacedOut")]
    [InlineData("(Not used)", "NotUsed")]
    [InlineData("%", "")]
    [InlineData("", "")]
    public void name_from_label(string label, string name)
    {
        Assert.Equal(name, FieldNames.FromLabel(label));
    }

    [Fact]
    public void colliding_name_gets_id()
    {
        var names = FieldNames.Assign(new[]
        {
            new NameCandidate("aare", "AreaOfEffect", false, new[] { "common", "AHbz" }),
            new NameCandidate("Hbz2", "AreaOfEffect", false, new[] { "AHbz" }),
            new NameCandidate("Hwe2", "AreaOfEffect", false, new[] { "AHwe" }),
            new NameCandidate("aran", "Range", true, new[] { "common", "AHbz" }),
            new NameCandidate("Hbz3", "Range", false, new[] { "AHbz" }),
        });

        Assert.Equal(new[] { "AreaOfEffectAare", "AreaOfEffectHbz2", "AreaOfEffect", "Range", "RangeHbz3" }, names);
    }

    [Fact]
    public void a_name_of_object_gets_its_id()
    {
        var names = FieldNames.Assign(new[] { new NameCandidate("utyp", "GetType", false, new[] { "unit" }) });

        Assert.Equal(new[] { "GetTypeUtyp" }, names);
    }

    [Theory]
    [InlineData("Declared")]
    [InlineData("Fields")]
    [InlineData("Levels")]
    [InlineData("PerLevel")]
    [InlineData("Set")]
    [InlineData("Totals")]
    public void a_name_of_a_kind_base_member_gets_its_id(string name)
    {
        var names = FieldNames.Assign(new[] { new NameCandidate("ulev", name, false, new[] { "unit" }) });

        Assert.Equal(new[] { name + "Ulev" }, names);
    }

    [Theory]
    [InlineData("Set")]
    [InlineData("Per Level")]
    public void a_label_that_gives_a_kind_base_member_gets_its_id(string label)
    {
        var name = FieldNames.FromLabel(label);
        var names = FieldNames.Assign(new[] { new NameCandidate("abcd", name, false, new[] { "all" }) });

        Assert.Equal(name + "Abcd", names[0]);
    }

    [Fact]
    public void two_listed_names_that_meet_are_an_error()
    {
        var error = Assert.Throws<InvalidOperationException>(() => FieldNames.Assign(new[]
        {
            new NameCandidate("uhpm", "Life", true, new[] { "unit" }),
            new NameCandidate("uhpr", "Life", true, new[] { "unit" }),
        }));

        Assert.Equal("fields uhpm and uhpr are both named Life", error.Message);
    }

    [Fact]
    public void a_name_of_a_runtime_member_gets_its_id()
    {
        var names = FieldNames.Assign(
            new[]
            {
                new NameCandidate("uhnd", "Handle", false, new[] { "unit" }),
                new NameCandidate("uply", "Player", false, new[] { "unit" }),
                new NameCandidate("uabc", "Other", false, new[] { "unit" }),
            },
            new HashSet<string> { "Handle", "Player" });

        Assert.Equal(new[] { "HandleUhnd", "PlayerUply", "Other" }, names);
    }

    [Fact]
    public void a_hand_picked_runtime_member_name_is_an_error()
    {
        var error = Assert.Throws<InvalidOperationException>(() => FieldNames.Assign(
            new[] { new NameCandidate("uhpm", "Handle", true, new[] { "unit" }) },
            new HashSet<string> { "Handle" }));

        Assert.Equal("field uhpm is hand-picked as Handle, a member of the class it belongs to", error.Message);
    }

    [Fact]
    public void a_shared_field_keeps_the_name_of_its_runtime_member()
    {
        var names = FieldNames.Assign(
            new[]
            {
                new NameCandidate("umpm", "MaxMana", true, new[] { "unit" }, Shared: true),
                new NameCandidate("umpr", "Handle", false, new[] { "unit" }),
            },
            new HashSet<string> { "MaxMana", "Handle" });

        Assert.Equal(new[] { "MaxMana", "HandleUmpr" }, names);
    }

    [Fact]
    public void fields_of_a_kind_avoid_the_reserved_names()
    {
        var tables = MiniExport();

        var plain = Find(Fields.Of(tables, Kind.Unit), "ubpx");
        var avoided = Find(Fields.Of(tables, Kind.Unit, new HashSet<string> { plain.Name }), "ubpx");

        Assert.Equal("ButtonPositionX", plain.Name);
        Assert.Equal("ButtonPositionXUbpx", avoided.Name);
    }

    [Fact]
    public void whole_number_limits_from_min_and_max()
    {
        var gold = Find(Fields.Of(MiniExport(), Kind.Item), "igol").Rules;
        var hitPoints = Find(Fields.Of(MiniExport(), Kind.Item), "ihtp").Rules;

        Assert.Equal(0, gold.Min);
        Assert.Equal(100000, gold.Max);
        Assert.Equal("Gold Cost", gold.Label);
        Assert.False(gold.NonNegative);
        Assert.False(gold.CanBeEmpty);
        Assert.Null(gold.MaxLength);
        Assert.Equal(1, hitPoints.Min);
        Assert.Equal(500000, hitPoints.Max);
        Assert.Equal("Hit Points", hitPoints.Label);
    }

    [Fact]
    public void an_unreal_field_is_non_negative_only_when_the_metadata_forces_it()
    {
        // uhpr is unreal with forceNonNeg 0: its minimum of 0 is part of its range, and the Phoenix's standard -25 is below it.
        var regeneration = Find(Fields.Of(MiniExport(), Kind.Unit), "uhpr").Rules;

        Assert.False(regeneration.NonNegative);
        Assert.False(regeneration.NoNegative);
        Assert.Equal(0, regeneration.Min);
        Assert.Equal(1000, regeneration.Max);
        Assert.False(Find(Fields.Of(MiniExport(), Kind.Unit), "uhpm").Rules.NonNegative);
    }

    [Fact]
    public void force_non_negative_is_read()
    {
        Assert.True(Find(Fields.Of(MiniExport(), Kind.Ability), "acdn").Rules.NonNegative);
        Assert.True(Find(Fields.Of(MiniExport(), Kind.Item), "isca").Rules.NonNegative);
    }

    [Fact]
    public void a_bool_row_has_no_limits()
    {
        var hero = Find(Fields.Of(MiniExport(), Kind.Ability), "aher").Rules;

        Assert.Null(hero.Min);
        Assert.Null(hero.Max);
        Assert.Equal("Hero Ability", hero.Label);
    }

    [Fact]
    public void a_string_limit_is_a_length_only_when_it_is_a_number()
    {
        var tables = MiniExport();

        // utip and unam name a text size (TTDesc, TTName), which is not a number of characters.
        Assert.Null(Find(Fields.Of(tables, Kind.Unit), "utip").Rules.MaxLength);
        Assert.Null(Find(Fields.Of(tables, Kind.Unit), "unam").Rules.MaxLength);
        Assert.Null(Find(Fields.Of(tables, Kind.Unit), "unam").Rules.Max);
        Assert.Equal(50, Find(Fields.Of(tables, Kind.Unit), "unsf").Rules.MaxLength);
        Assert.Null(Find(Fields.Of(tables, Kind.Unit), "unsf").Rules.MaxEntries);
    }

    [Fact]
    public void a_list_limit_is_a_number_of_entries()
    {
        var tables = MiniExport();

        // A stringList's maxVal counts entries: the standard upro holds 279 characters in 18 names under a maxVal of 32.
        var casterNames = Find(Fields.Of(tables, Kind.Unit), "ucun").Rules;
        Assert.Equal(32, casterNames.MaxEntries);
        Assert.Null(casterNames.MaxLength);
        Assert.Null(casterNames.Max);
        Assert.Equal(12, Find(Fields.Of(tables, Kind.Unit), "usei").Rules.MaxEntries);
        Assert.Equal(5, Find(Fields.Of(tables, Kind.Unit), "uhab").Rules.MaxEntries);
        Assert.Null(Find(Fields.Of(tables, Kind.Unit), "uabi").Rules.MaxEntries);
        Assert.Null(Find(Fields.Of(tables, Kind.Unit), "usei").Rules.Max);
    }

    [Fact]
    public void only_a_name_is_required()
    {
        var tables = MiniExport();

        Assert.True(Find(Fields.Of(tables, Kind.Unit), "unam").Rules.Required);
        Assert.True(Find(Fields.Of(tables, Kind.Ability), "anam").Rules.Required);
        Assert.True(Find(Fields.Of(tables, Kind.Destructable), "bnam").Rules.Required);
        Assert.True(Find(Fields.Of(tables, Kind.Upgrade), "gnam").Rules.Required);

        // canBeEmpty is 0 on these too, yet standard objects leave them empty: 267 units have no abilities.
        Assert.False(Find(Fields.Of(tables, Kind.Unit), "ucun").Rules.Required);
        Assert.False(Find(Fields.Of(tables, Kind.Unit), "uabi").Rules.Required);
        Assert.False(Find(Fields.Of(tables, Kind.Unit), "usei").Rules.Required);
        Assert.False(Find(Fields.Of(tables, Kind.Unit), "utip").Rules.Required);
        Assert.False(Find(Fields.Of(tables, Kind.Unit), "uhpm").Rules.Required);
    }

    [Theory]
    [InlineData(1d, null, false, false, false)]
    [InlineData(0d, 10d, false, false, false)]
    [InlineData(0d, 10d, true, false, true)]
    [InlineData(0d, 10d, true, true, false)]
    [InlineData(-1000d, 1000d, true, false, false)]
    [InlineData(null, 9d, false, false, false)]
    [InlineData(null, null, true, false, true)]
    [InlineData(null, null, true, true, false)]
    public void no_negative_only_when_the_metadata_forces_it_and_no_standard_value_is_negative(double? min, double? max, bool nonNegative, bool standardNegative, bool expected)
    {
        var rules = new FieldRules(min, max, nonNegative, false, null, null, StandardNegative: standardNegative);

        Assert.Equal(expected, rules.NoNegative);
    }

    [Fact]
    public void a_negative_standard_value_in_the_base_tables_or_a_balance_set_lets_the_field_hold_negatives()
    {
        var forced = new FieldRules(0, 10, true, false, null, null);
        Field Number(string id, FieldType type = FieldType.Int)
        {
            return new Field(Kind.Unit, id, id, type, null, false, 0, false, Array.Empty<string>(), id, Rules: forced);
        }

        var fields = new[]
        {
            Number("zab1"), Number("zab2"), Number("zab3", FieldType.Float), Number("zab4", FieldType.String), Number("zab5"),
            Number("zab6") with { Rules = FieldRules.None },
        };
        IReadOnlyDictionary<string, IReadOnlyDictionary<string, string>> NoSets() => new Dictionary<string, IReadOnlyDictionary<string, string>>();
        var objects = new[]
        {
            new StandardObject(Kind.Unit, "h000", "", "", false, false, null,
                new Dictionary<string, string> { ["zab1"] = "-11", ["zab2"] = "3", ["zab4"] = "1,-2", ["zab5:2"] = "4", ["zab6"] = "-1" },
                NoSets(), Array.Empty<string>()),
            new StandardObject(Kind.Unit, "h001", "", "", false, false, null,
                new Dictionary<string, string> { ["zab2"] = "4", ["zab5:1"] = "0" },
                new Dictionary<string, IReadOnlyDictionary<string, string>>
                {
                    ["custom_v1"] = new Dictionary<string, string> { ["zab3"] = "-0.6", ["zab2"] = "" },
                },
                Array.Empty<string>()),
        };

        var result = Fields.WithStandardNegatives(fields, objects).ToDictionary(field => field.Id);

        Assert.False(result["zab1"].Rules.NoNegative);
        Assert.True(result["zab1"].Rules.StandardNegative);
        Assert.True(result["zab2"].Rules.NoNegative);
        Assert.False(result["zab3"].Rules.NoNegative);
        Assert.False(result["zab4"].Rules.NoNegative);
        Assert.True(result["zab5"].Rules.NoNegative);
        Assert.Same(FieldRules.None, result["zab6"].Rules);
    }

    [Fact]
    public void can_be_empty_is_read()
    {
        var tables = MiniExport();

        Assert.True(Find(Fields.Of(tables, Kind.Unit), "utip").Rules.CanBeEmpty);
        Assert.False(Find(Fields.Of(tables, Kind.Unit), "unam").Rules.CanBeEmpty);
        Assert.False(Find(Fields.Of(tables, Kind.Unit), "ucun").Rules.CanBeEmpty);
    }

    [Fact]
    public void a_field_made_without_rules_has_empty_ones()
    {
        var field = new Field(Kind.Unit, "uhpm", "Life", FieldType.Int, null, false, 0, false, Array.Empty<string>(), "HP");

        Assert.Equal(FieldRules.None, field.Rules);
        Assert.Null(field.Rules.Min);
        Assert.Null(field.Rules.Label);
    }

    [Fact]
    public void a_hero_only_unit_field_is_marked()
    {
        var fields = Fields.Of(MiniExport(), Kind.Unit);

        Assert.True(Find(fields, "ustr").HeroOnly);
        Assert.True(Find(fields, "uhab").HeroOnly);
        Assert.False(Find(fields, "uhpm").HeroOnly);
        Assert.False(Find(fields, "uabi").HeroOnly);
        Assert.All(Fields.Of(MiniExport(), Kind.Ability), f => Assert.False(f.HeroOnly));
    }
}
