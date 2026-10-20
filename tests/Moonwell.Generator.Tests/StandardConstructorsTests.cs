using Moonwell.Generator.Objects;

namespace Moonwell.Generator.Tests;

public class StandardConstructorsTests
{
    private const string Abilities = "Moonwell.Objects.Human.Abilities";
    private const string Units = "Moonwell.Objects.Human.Units";

    private static readonly Lazy<IReadOnlyDictionary<string, string>> MiniFiles = new(
        () => GeneratedObjects.Make(MiniExport(), RuntimeMembers.Read(Path.Combine(AppContext.BaseDirectory, "Moonwell.dll")), "3.0.0.24268"));

    private static readonly ChoiceType AttackTypes = new(
        "attackType",
        "AttackType",
        ChoiceForm.One,
        "string",
        new[]
        {
            new ChoiceMember("Normal", 0, "normal", Array.Empty<string>(), "Normal"),
            new ChoiceMember("Hero", 1, "hero", Array.Empty<string>(), "Hero"),
        });

    private static readonly ChoiceType Targets = new(
        "targetList",
        "Targets",
        ChoiceForm.List,
        "string",
        new[]
        {
            new ChoiceMember("Air", 1, "air", Array.Empty<string>(), "Air"),
            new ChoiceMember("Enemies", 2, "enemies", new[] { "enem" }, "Enemies"),
            new ChoiceMember("Ground", 4, "ground", Array.Empty<string>(), "Ground"),
        });

    private static readonly IReadOnlyList<Field> UnitFields = new[]
    {
        Plain("uhpm", "HitPoints", FieldType.Int),
        Plain("ua1c", "AttackCooldown", FieldType.Float),
        Plain("unam", "Name", FieldType.String),
        Plain("udro", "Dropped", FieldType.Bool),
        Plain("uabi", "Abilities", FieldType.ObjectList) with { ListOf = "Ability" },
        Plain("utra", "Trains", FieldType.ObjectList) with { ListOf = "Unit" },
        Plain("udaa", "DeathAbility", FieldType.ObjectRef) with { ListOf = "Ability" },
        Plain("ua1t", "AttackType", FieldType.Choice) with { Choice = AttackTypes },
        Plain("ua1g", "TargetsAllowed", FieldType.Choice) with { Choice = Targets },
        Plain("upro", "ProperNames", FieldType.String),
    };

    private static readonly IReadOnlyList<Field> AbilityFields = new[]
    {
        Plain("acdn", "Cooldown", FieldType.Float, Kind.Ability) with { PerLevel = true },
        Plain("atp1", "Tooltip", FieldType.String, Kind.Ability) with { PerLevel = true },
        Plain("atar", "AbilityTargets", FieldType.Choice, Kind.Ability) with { PerLevel = true, Choice = Targets },
    };

    private static Field Plain(string id, string name, FieldType type, Kind kind = Kind.Unit)
    {
        return new Field(kind, id, name, type, null, false, 0, false, Array.Empty<string>(), id);
    }

    private static StandardObject Object(Kind kind, string id, IReadOnlyDictionary<string, string> values)
    {
        return new StandardObject(
            kind,
            id,
            id,
            "human",
            false,
            kind == Kind.Unit && char.IsAsciiLetterUpper(id[0]),
            kind == Kind.Ability ? id : null,
            values,
            new Dictionary<string, IReadOnlyDictionary<string, string>>(),
            Array.Empty<string>());
    }

    // Ten abilities A000 to A009 with classes, and a footman.
    private static StandardConstructors Constructors()
    {
        var classes = new List<(StandardObject, ClassName)>();
        for (var i = 0; i < 10; i++)
        {
            classes.Add((Object(Kind.Ability, "A00" + i, new Dictionary<string, string>()), new ClassName(Abilities, "Ability" + i)));
        }

        classes.Add((Object(Kind.Unit, "hfoo", new Dictionary<string, string>()), new ClassName(Units, "Footman")));
        var fields = new Dictionary<Kind, IReadOnlyList<Field>> { [Kind.Unit] = UnitFields, [Kind.Ability] = AbilityFields };
        return new StandardConstructors(fields, classes);
    }

    private static IReadOnlyList<string> UnitLines(Dictionary<string, string> values)
    {
        return Constructors().Lines(Object(Kind.Unit, "hkni", values), Array.Empty<Field>());
    }

    private static ExportTables MiniExport()
    {
        return ExportTables.Read(Path.GetFullPath(
            Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Fixtures", "MiniExport", "war3.w3mod")));
    }

    // The source of a class: from its declaration to its closing brace.
    private static string Class(string source, string name)
    {
        var start = source.IndexOf($" class {name} ", StringComparison.Ordinal);
        Assert.True(start >= 0, $"no class {name} in:\n{source}");
        var lineStart = source.LastIndexOf('\n', start) + 1;
        var indent = new string(' ', source.Substring(lineStart).TakeWhile(ch => ch == ' ').Count());
        var end = source.IndexOf("\n" + indent + "}\n", start, StringComparison.Ordinal);
        return source.Substring(start, end + 1 - start);
    }

    [Fact]
    public void the_paladin_sets_the_base_tables_values_in_its_constructor()
    {
        var paladin = Class(MiniFiles.Value["Human/Units.g.cs"], "Paladin");

        Assert.Contains("                public Paladin()\n                {\n", paladin);
        Assert.Contains("                    HitPoints = 100;\n", paladin);
        Assert.Contains("                    AttackCooldown = 2f;\n", paladin);
        Assert.Contains("                    AgilityPerLevel = 1.5f;\n", paladin);
        Assert.Contains("                    ManaRegen = 0.01f;\n", paladin);
        Assert.Contains("                    Name = \"Paladin\";\n", paladin);
        Assert.Contains("                    Icon = \"ReplaceableTextures\\\\CommandButtons\\\\BTNHeroPaladin.blp\";\n", paladin);
        Assert.Contains("                    AttackType = global::Moonwell.AttackType.Hero;\n", paladin);
        Assert.Contains("                    PrimaryAttribute = global::Moonwell.Objects.AttributeType.STR;\n", paladin);
        // A list names the standard classes; an id with no class in the export is added by its id.
        Assert.Contains(
            "                    HeroAbilities.Set<global::Moonwell.Objects.Human.Abilities.HolyLight>();\n"
            + "                    HeroAbilities.Add(\"AHds\");\n"
            + "                    HeroAbilities.Add(\"AHre\");\n"
            + "                    HeroAbilities.Add(\"AHad\");\n",
            paladin);
        Assert.Contains("                    RequirementsTier2.Add(\"hkee\");\n", paladin);
        // A field with no value in the base tables gets no line.
        Assert.DoesNotContain("Description =", paladin);
    }

    [Fact]
    public void an_abilitys_levels_and_own_fields_are_set_in_its_constructor()
    {
        var bolt = Class(MiniFiles.Value["Human/Abilities.g.cs"], "StormBolt");

        Assert.Contains("                public StormBolt()\n                {\n", bolt);
        Assert.Contains("                    Damage = Levels(100f, 200f, 310f);\n", bolt);
        Assert.Contains("                    Cooldown = Levels(9f, 9f, 9f);\n", bolt);
        // Only level 1 has a value in the tables.
        Assert.Contains("                    CastTime = Levels(0f);\n", bolt);
        Assert.Contains("                    Tooltip = Levels(\"Storm Bolt - [|cffffcc00Level 1|r]\", \"Storm Bolt - [|cffffcc00Level 2|r]\", \"Storm Bolt - [|cffffcc00Level 3|r]\");\n", bolt);
        Assert.Contains(
            "                    TargetsAllowed = Levels("
            + "global::Moonwell.Objects.Targets.Air | global::Moonwell.Objects.Targets.Enemies | global::Moonwell.Objects.Targets.Ground, "
            + "global::Moonwell.Objects.Targets.Air | global::Moonwell.Objects.Targets.Enemies | global::Moonwell.Objects.Targets.Ground, "
            + "global::Moonwell.Objects.Targets.Air | global::Moonwell.Objects.Targets.Enemies | global::Moonwell.Objects.Targets.Ground);\n",
            bolt);
        // A list of each level is set as its property reads it, from level 1.
        Assert.Contains("                    Buffs.Add(\"BPSE\");\n", bolt);
        Assert.Contains("                    LevelCount = 3;\n", bolt);
    }

    [Fact]
    public void the_constructor_comes_before_an_abilitys_own_properties()
    {
        var bolt = Class(MiniFiles.Value["Human/Abilities.g.cs"], "StormBolt");

        Assert.True(bolt.IndexOf("public StormBolt()", StringComparison.Ordinal) < bolt.IndexOf("[global::Moonwell.Objects.Field(", StringComparison.Ordinal));
    }

    [Fact]
    public void lines_are_ordered_by_property_name()
    {
        var lines = UnitLines(new Dictionary<string, string> { ["uhpm"] = "420", ["unam"] = "Knight", ["ua1c"] = "1.5" });

        Assert.Equal(new[] { "AttackCooldown = 1.5f;", "HitPoints = 420;", "Name = \"Knight\";" }, lines);
    }

    [Fact]
    public void values_read_as_the_net_side_reads_them()
    {
        var lines = UnitLines(new Dictionary<string, string>
        {
            ["uhpm"] = "-7",
            ["ua1c"] = "1E-05",
            ["udro"] = "1",
            ["ua1t"] = "Hero",
            ["ua1g"] = "enem,air,debris",
        });

        Assert.Equal(
            new[]
            {
                "AttackCooldown = 1E-05f;",
                "AttackType = global::Moonwell.AttackType.Hero;",
                "Dropped = true;",
                "HitPoints = -7;",
                "TargetsAllowed = global::Moonwell.Objects.Targets.Air | global::Moonwell.Objects.Targets.Enemies;",
            },
            lines);
    }

    [Fact]
    public void a_number_the_net_side_cannot_read_whole_is_truncated_and_a_text_that_is_none_is_0()
    {
        var lines = UnitLines(new Dictionary<string, string> { ["uhpm"] = "12.75", ["ua1c"] = "fast", ["udro"] = "0" });

        Assert.Equal(new[] { "AttackCooldown = 0f;", "Dropped = false;", "HitPoints = 12;" }, lines);
    }

    [Fact]
    public void a_choice_with_no_member_of_its_number_is_cast_and_no_flags_is_0()
    {
        var lines = UnitLines(new Dictionary<string, string> { ["ua1t"] = "-3", ["ua1g"] = "debris" });

        Assert.Equal(new[] { "AttackType = (global::Moonwell.AttackType)(-3);", "TargetsAllowed = 0;" }, lines);
    }

    [Fact]
    public void no_flags_in_levels_keeps_its_cast_which_gives_the_type()
    {
        var lines = Constructors().Lines(Object(Kind.Ability, "A000", new Dictionary<string, string> { ["atar:1"] = "debris", ["atar:2"] = "air" }), AbilityFields);

        Assert.Contains("AbilityTargets = Levels((global::Moonwell.Objects.Targets)0, global::Moonwell.Objects.Targets.Air);", lines);
    }

    [Fact]
    public void a_text_is_escaped_as_a_csharp_literal()
    {
        var lines = UnitLines(new Dictionary<string, string> { ["unam"] = "a \"b\" \\c\nd\te\u2028" });

        Assert.Equal(new[] { "Name = \"a \\\"b\\\" \\\\c\\nd\\te\\u2028\";" }, lines);
    }

    [Fact]
    public void a_list_longer_than_eight_sets_eight_and_adds_the_rest_in_order()
    {
        var lines = UnitLines(new Dictionary<string, string> { ["uabi"] = "A000,A001,A002,A003,A004,A005,A006,A007,A008,A009" });

        Assert.Equal(
            new[]
            {
                "Abilities.Set<global::Moonwell.Objects.Human.Abilities.Ability0, global::Moonwell.Objects.Human.Abilities.Ability1, "
                + "global::Moonwell.Objects.Human.Abilities.Ability2, global::Moonwell.Objects.Human.Abilities.Ability3, "
                + "global::Moonwell.Objects.Human.Abilities.Ability4, global::Moonwell.Objects.Human.Abilities.Ability5, "
                + "global::Moonwell.Objects.Human.Abilities.Ability6, global::Moonwell.Objects.Human.Abilities.Ability7>();",
                "Abilities.Add<global::Moonwell.Objects.Human.Abilities.Ability8>();",
                "Abilities.Add<global::Moonwell.Objects.Human.Abilities.Ability9>();",
            },
            lines);
    }

    [Fact]
    public void an_id_with_no_class_of_the_lists_kind_is_added_by_its_id_in_its_place()
    {
        // zzzz has no class; hfoo has one, but it is a unit, not an ability.
        var lines = UnitLines(new Dictionary<string, string> { ["uabi"] = "zzzz,A001,hfoo,A002", ["utra"] = "hfoo" });

        Assert.Equal(
            new[]
            {
                "Abilities.Add(\"zzzz\");",
                "Abilities.Add<global::Moonwell.Objects.Human.Abilities.Ability1>();",
                "Abilities.Add(\"hfoo\");",
                "Abilities.Add<global::Moonwell.Objects.Human.Abilities.Ability2>();",
                "Trains.Set<global::Moonwell.Objects.Human.Units.Footman>();",
            },
            lines);
    }

    [Fact]
    public void a_reference_is_set_to_its_class_or_its_id()
    {
        Assert.Equal(
            new[] { "DeathAbility.Set<global::Moonwell.Objects.Human.Abilities.Ability3>();" },
            UnitLines(new Dictionary<string, string> { ["udaa"] = "A003" }));
        Assert.Equal(new[] { "DeathAbility.Set(\"hfoo\");" }, UnitLines(new Dictionary<string, string> { ["udaa"] = "hfoo" }));
    }

    [Fact]
    public void levels_with_a_gap_are_set_one_level_at_a_time()
    {
        var lines = Constructors().Lines(
            Object(Kind.Ability, "A000", new Dictionary<string, string> { ["acdn:1"] = "5", ["acdn:3"] = "7.5", ["atp1:1"] = "One", ["atp1:2"] = "Two" }),
            Array.Empty<Field>());

        Assert.Equal(new[] { "Set(\"acdn\", 1, 5f);", "Set(\"acdn\", 3, 7.5f);", "Tooltip = Levels(\"One\", \"Two\");" }, lines);
    }

    [Fact]
    public void a_value_of_a_field_the_class_does_not_have_is_an_error()
    {
        var error = Assert.Throws<InvalidOperationException>(() => UnitLines(new Dictionary<string, string> { ["zzzz"] = "1" }));

        Assert.Equal("hkni has a value for zzzz, which its class has no property for", error.Message);
    }

    [Fact]
    public void an_object_with_no_values_has_no_constructor()
    {
        var tables = MiniExport();
        var item = StandardObjects.Of(tables, Kind.Item, Fields.Of(tables, Kind.Item)).Single(o => o.Id == "ratf");

        var text = ObjectsWriter.Group(
            "Moonwell.Objects.Items",
            new[] { (item with { Values = new Dictionary<string, string>() }, new ClassName("Moonwell.Objects.Items", "Empty")) },
            Array.Empty<Field>(),
            Constructors());

        Assert.DoesNotContain("public Empty()", text);
    }
}
