using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Moonwell.Generator.Objects;

namespace Moonwell.Generator.Tests;

public class ChoicesTests
{
    private static ExportTables MiniExport()
    {
        return ExportTables.Read(Path.GetFullPath(
            Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Fixtures", "MiniExport", "war3.w3mod")));
    }

    private static ChoiceEntry Entry(int index, string text, string label = "", params string[] alternates)
    {
        return new ChoiceEntry(index, text, label, alternates);
    }

    private static string Label(string key)
    {
        return key;
    }

    [Fact]
    public void the_editor_data_gives_a_types_values_in_order_with_their_other_spellings()
    {
        var targets = MiniExport().Choices("targetList")!;

        Assert.Equal(new[] { "air", "alive", "allies", "enemies", "ground", "notself" }, targets.Select(e => e.Text));
        Assert.Equal(new[] { 0, 1, 2, 3, 4, 5 }, targets.Select(e => e.Index));
        Assert.Equal("WESTRING_UE_TARGETTYPE_AIR", targets[0].Label);
        Assert.Empty(targets[0].Alternates);
        Assert.Equal(new[] { "aliv" }, targets[1].Alternates);
        Assert.Equal(new[] { "alli", "ally" }, targets[2].Alternates);
        Assert.Null(MiniExport().Choices("unitRace"));
        Assert.Null(MiniExport().Choices("int"));
    }

    [Fact]
    public void a_choice_of_words_is_numbered_in_order_and_named_by_its_words()
    {
        var attack = Choices.Of(MiniExport(), "attackType", "string")!;

        Assert.Equal("AttackType", attack.Name);
        Assert.Equal(ChoiceForm.One, attack.Form);
        Assert.False(attack.Flags);
        Assert.Equal(new[] { "Unknown", "Normal", "Pierce", "Siege", "Spells", "Chaos", "Magic", "Hero" }, attack.Members.Select(m => m.Name));
        Assert.Equal(Enumerable.Range(0, 8), attack.Members.Select(m => m.Value));
        Assert.Equal("magic", attack.Members[6].Text);
        Assert.Equal("None", attack.Members[0].Label);
    }

    [Fact]
    public void a_choice_of_numbers_has_those_numbers_and_is_named_by_its_labels()
    {
        var death = Choices.Of(MiniExport(), "deathType", "int")!;

        Assert.Equal("DeathType", death.Name);
        Assert.Equal(ChoiceForm.One, death.Form);
        Assert.Equal(
            new[] { "CantRaiseDoesNotDecay", "CanRaiseDoesNotDecay", "CantRaiseDoesDecay", "CanRaiseDoesDecay" },
            death.Members.Select(m => m.Name));
        Assert.Equal(new[] { 0, 1, 2, 3 }, death.Members.Select(m => m.Value));
        Assert.Equal(new[] { "0", "1", "2", "3" }, death.Members.Select(m => m.Text));
        Assert.Equal("Can raise, Does decay", death.Members[3].Label);
    }

    [Fact]
    public void numbers_out_of_order_keep_their_numbers()
    {
        // defenseTypeInt: the game's own numbers for the defense types, which World Editor lists in another order.
        var defense = Choices.Make("defenseTypeInt", "int", new[] { Entry(0, "4", "Normal"), Entry(1, "0", "Small"), Entry(2, "1", "Medium") }, Label);

        Assert.Equal(new[] { ("Normal", 4), ("Small", 0), ("Medium", 1) }, defense.Members.Select(m => (m.Name, m.Value)));
    }

    [Fact]
    public void a_list_type_is_flags_with_a_bit_for_each_value()
    {
        var targets = Choices.Of(MiniExport(), "targetList", "string")!;

        Assert.Equal("Targets", targets.Name);
        Assert.Equal(ChoiceForm.List, targets.Form);
        Assert.True(targets.Flags);
        Assert.Equal(new[] { "Air", "Alive", "Allies", "Enemies", "Ground", "NotSelf" }, targets.Members.Select(m => m.Name));
        Assert.Equal(new[] { 1, 2, 4, 8, 16, 32 }, targets.Members.Select(m => m.Value));
        Assert.Equal(new[] { "aliv" }, targets.Members[1].Alternates);
        Assert.Equal("Air", targets.Members[0].Label);
        Assert.Equal("Allied", targets.Members[2].Label);
    }

    [Theory]
    [InlineData("unitClass")]
    [InlineData("abilityModifierStats")]
    [InlineData("pathingListPrevent")]
    public void types_whose_values_the_data_lists_are_flags(string type)
    {
        var choice = Choices.Make(type, "string", new[] { Entry(0, "giant", "Giant"), Entry(1, "undead", "Undead") }, Label);

        Assert.Equal(ChoiceForm.List, choice.Form);
        Assert.Equal(new[] { 1, 2 }, choice.Members.Select(m => m.Value));
    }

    [Fact]
    public void a_flags_type_stored_as_a_number_has_the_bits_its_values_number()
    {
        var morph = Choices.Of(MiniExport(), "morphFlags", "int")!;

        Assert.Equal("MorphFlags", morph.Name);
        Assert.Equal(ChoiceForm.Bits, morph.Form);
        Assert.True(morph.Flags);
        Assert.Equal(new[] { "Uninterruptable", "ImmediateLanding", "ImmediateTakeOff" }, morph.Members.Select(m => m.Name));
        Assert.Equal(new[] { 1, 2, 4 }, morph.Members.Select(m => m.Value));
        Assert.Equal(new[] { "1", "2", "4" }, morph.Members.Select(m => m.Text));
    }

    [Fact]
    public void full_flags_is_one_choice_of_numbers()
    {
        // Never, Life Only, Mana Only, Always: Rejuvenation's standard 3 is Always, not the bits of Never and Life Only.
        var full = Choices.Make(
            "fullFlags",
            "int",
            new[] { Entry(0, "0", "Never"), Entry(1, "1", "Life Only"), Entry(2, "2", "Mana Only"), Entry(3, "3", "Always") },
            Label);

        Assert.Equal(ChoiceForm.One, full.Form);
        Assert.Equal(new[] { ("Never", 0), ("LifeOnly", 1), ("ManaOnly", 2), ("Always", 3) }, full.Members.Select(m => (m.Name, m.Value)));
    }

    [Fact]
    public void a_word_with_a_digit_or_no_letter_is_named_by_its_label()
    {
        var stats = Choices.Make("abilityModifierStats", "string", new[] { Entry(0, "AS01", "Health Regeneration") }, Label);
        var buffer = Choices.Make("aiBuffer", "string", new[] { Entry(0, "_", "None"), Entry(1, "townhall", "Hall") }, Label);
        var race = Choices.Make("unitRace", "string", new[] { Entry(0, "nightelf", "Night Elf"), Entry(1, "creeps", "Creep") }, Label);

        Assert.Equal("HealthRegeneration", stats.Members[0].Name);
        Assert.Equal("AS01", stats.Members[0].Text);
        Assert.Equal(new[] { "None", "Townhall" }, buffer.Members.Select(m => m.Name));
        Assert.Equal(new[] { "NightElf", "Creeps" }, race.Members.Select(m => m.Name));
        Assert.Equal("AiBuffer", buffer.Name);
    }

    [Fact]
    public void a_flags_type_of_more_than_31_values_is_an_error()
    {
        var entries = Enumerable.Range(0, 32).Select(i => Entry(i, "v" + i, "V" + i)).ToArray();

        var error = Assert.Throws<InvalidOperationException>(() => Choices.Make("targetList", "string", entries, Label));

        Assert.Contains("targetList", error.Message);
        Assert.Contains("31", error.Message);
    }

    [Fact]
    public void two_values_of_one_name_are_an_error()
    {
        var error = Assert.Throws<InvalidOperationException>(
            () => Choices.Make("regenType", "string", new[] { Entry(0, "day", "Day"), Entry(1, "Day", "Day") }, Label));

        Assert.Equal("regenType: two values are named Day", error.Message);
    }

    [Fact]
    public void every_used_type_with_a_section_is_a_choice_and_a_name_taken_gets_a_suffix()
    {
        var all = Choices.All(MiniExport(), null);

        // The mini export's metadata uses attackType, attributeType and targetList; deathType and morphFlags have sections but no field.
        Assert.Equal(new[] { "attackType", "attributeType", "targetList" }, all.Keys.OrderBy(k => k, StringComparer.Ordinal));
        Assert.Equal("Targets", all["targetList"].Name);
        Assert.Equal("AttributeTypeChoice", Choices.All(MiniExport(), new HashSet<string> { "AttributeType" })["attributeType"].Name);
    }

    [Fact]
    public void attack_and_defense_types_are_core_moonwells_enums()
    {
        // The damage package names them too, and it cannot reference Moonwell.Objects (C19): a map has one of each, in core.
        var attack = Choices.Of(MiniExport(), "attackType", "string")!;
        var defense = Choices.Make("defenseType", "string", new[] { Entry(0, "normal", "Normal"), Entry(1, "divine", "Divine") }, Label);
        var numbered = Choices.Make("defenseTypeInt", "int", new[] { Entry(0, "4", "Normal") }, Label);

        Assert.True(attack.Core);
        Assert.Equal("global::Moonwell.AttackType", attack.TypeName);
        Assert.True(defense.Core);
        Assert.Equal("global::Moonwell.DefenseType", defense.TypeName);
        Assert.False(numbered.Core);
        Assert.Equal("global::Moonwell.Objects.DefenseTypeInt", numbered.TypeName);

        // Namespace Moonwell has them, so their names are taken there; they keep them.
        Assert.Equal("AttackType", Choices.All(MiniExport(), new HashSet<string> { "AttackType" })["attackType"].Name);
    }

    [Fact]
    public void a_core_enum_has_the_members_the_game_data_gives()
    {
        var attack = Choices.Of(MiniExport(), "attackType", "string")!;
        var members = attack.Members.Select(m => (m.Name, m.Value)).ToList();

        Choices.CheckCore(attack, members);
        var renumbered = members.Select(m => m.Name == "Magic" ? (m.Name, 4) : m).ToList();
        var error = Assert.Throws<InvalidOperationException>(() => Choices.CheckCore(attack, renumbered));
        var missing = Assert.Throws<InvalidOperationException>(() => Choices.CheckCore(attack, null));

        Assert.Equal(
            "attackType: Moonwell.AttackType has Unknown = 0, Normal = 1, Pierce = 2, Siege = 3, Spells = 4, Chaos = 5, Magic = 4, Hero = 7; "
            + "the game's data gives Unknown = 0, Normal = 1, Pierce = 2, Siege = 3, Spells = 4, Chaos = 5, Magic = 6, Hero = 7",
            error.Message);
        Assert.Equal(
            "attackType: Moonwell has no enum AttackType; the game's data gives Unknown = 0, Normal = 1, Pierce = 2, Siege = 3, Spells = 4, Chaos = 5, Magic = 6, Hero = 7",
            missing.Message);
    }

    [Fact]
    public void a_choice_field_holds_its_enum()
    {
        var tables = MiniExport();

        var attack = Fields.Of(tables, Kind.Unit).Single(f => f.Id == "ua1t");
        var targets = Fields.Of(tables, Kind.Ability).Single(f => f.Id == "atar");

        Assert.Equal(FieldType.Choice, attack.Type);
        Assert.Equal("AttackType", attack.Choice!.Name);
        Assert.Equal("string", attack.Store);
        Assert.Equal(FieldType.Choice, targets.Type);
        Assert.Equal("Targets", targets.Choice!.Name);
        Assert.Null(Fields.Of(tables, Kind.Unit).Single(f => f.Id == "uhpm").Choice);
    }

    [Fact]
    public void a_property_named_like_an_enum_it_does_not_hold_is_renamed()
    {
        // Inside a class, a property named Targets that holds Levels<Targets> hides the enum: Targets.Air would not compile.
        var ability = Fields.Of(MiniExport(), Kind.Ability).Single(f => f.Id == "atar");

        Assert.Equal("TargetsAllowed", ability.Name);
    }

    [Fact]
    public void a_hand_picked_name_of_an_enum_it_does_not_hold_is_an_error()
    {
        var targets = Choices.Of(MiniExport(), "targetList", "string")!;
        var enums = new HashSet<string> { "Targets" };

        var error = Assert.Throws<InvalidOperationException>(() => Fields.BesideEnums("atar", "Targets", true, targets, true, enums));

        Assert.Equal("field atar is hand-picked as Targets, the name of an enum its property does not hold", error.Message);
        Assert.Equal("TargetsAtar", Fields.BesideEnums("atar", "Targets", false, targets, true, enums));
        Assert.Equal("Targets", Fields.BesideEnums("utar", "Targets", true, targets, false, enums));
        Assert.Equal("Name", Fields.BesideEnums("unam", "Name", false, null, false, enums));
    }

    [Fact]
    public void the_enums_source_has_a_game_value_for_each_member()
    {
        var source = ObjectsWriter.Choices(Choices.All(MiniExport(), null).Values);

        Assert.Contains(
            "    /// <summary>A value of the metadata type <c>attributeType</c>, as World Editor offers it. Its numbers are Moonwell's, from World Editor's list, "
            + "not the natives' (JASS's ATTACK_TYPE_MAGIC is 4, AttackType.Magic is 6): never cast one to or from a native's value.</summary>\n"
            + "    /// @CSharpLua.Ignore\n"
            + "    public enum AttributeType\n"
            + "    {\n"
            + "        /// <summary>Agility.</summary>\n"
            + "        [global::Moonwell.Objects.GameValue(\"AGI\")]\n"
            + "        AGI = 0,\n",
            source);
        Assert.Contains(
            "    /// @CSharpLua.Ignore\n"
            + "    [global::System.Flags]\n"
            + "    public enum Targets\n",
            source);
        Assert.Contains(
            "        /// <summary>Alive.</summary>\n"
            + "        [global::Moonwell.Objects.GameValue(\"alive\", \"aliv\")]\n"
            + "        Alive = 2,\n",
            source);
        Assert.True(source.IndexOf("enum AttributeType", StringComparison.Ordinal) < source.IndexOf("enum Targets", StringComparison.Ordinal));
    }

    [Fact]
    public void a_core_enums_game_values_are_a_table_beside_the_others()
    {
        // The enum is Moonwell.AttackType; Moonwell.Objects keeps the texts of its members, which the map's Lua never needs.
        var source = ObjectsWriter.Choices(Choices.All(MiniExport(), null).Values);

        Assert.DoesNotContain("public enum AttackType", source);
        Assert.Contains(
            "    /// @CSharpLua.Ignore\n"
            + "    internal static class CoreChoices\n"
            + "    {\n"
            + "        /// <summary>The game's texts of <see cref=\"global::Moonwell.AttackType\"/>, the metadata type <c>attackType</c>: each member is the core enum's member of its name and number.</summary>\n"
            + "        internal enum AttackType\n"
            + "        {\n"
            + "            /// <summary>None.</summary>\n"
            + "            [global::Moonwell.Objects.GameValue(\"unknown\")]\n"
            + "            Unknown = 0,\n",
            source);
        Assert.True(source.IndexOf("enum Targets", StringComparison.Ordinal) < source.IndexOf("class CoreChoices", StringComparison.Ordinal));
    }

    [Fact]
    public void a_flags_member_named_none_says_it_is_not_0()
    {
        var targets = Choices.Make("targetList", "string", new[] { Entry(0, "air", "Air"), Entry(1, "none", "None") }, Label);

        var source = ObjectsWriter.Choices(new[] { targets });

        Assert.Contains("and 0 is none of them.", source);
        Assert.Contains(
            "        /// <summary>None. World Editor's \"none\", a value of its own: no values at all is 0, not this.</summary>\n"
            + "        [global::Moonwell.Objects.GameValue(\"none\")]\n"
            + "        None = 2,\n",
            source);
    }

    [Fact]
    public void a_bit_the_standard_data_uses_that_world_editor_does_not_list_is_a_member()
    {
        var versions = Choices.Make("versionFlags", "int", new[] { Entry(0, "0", "Reign of Chaos"), Entry(1, "1", "The Frozen Throne") }, Label);
        var attack = Choices.Make("attackType", "string", new[] { Entry(0, "normal", "Normal") }, Label);
        var choices = new Dictionary<string, ChoiceType> { ["versionFlags"] = versions, ["attackType"] = attack };
        var fields = new[]
        {
            new Field(Kind.Unit, "uver", "Versions", FieldType.Choice, null, false, 0, false, Array.Empty<string>(), "uver", Choice: versions),
        };
        var objects = new[]
        {
            new StandardObject(Kind.Unit, "hfoo", "", "", false, false, null, new Dictionary<string, string> { ["uver"] = "6" },
                new Dictionary<string, IReadOnlyDictionary<string, string>> { ["custom_v1"] = new Dictionary<string, string> { ["uver"] = "16" } },
                Array.Empty<string>()),
        };

        var result = Choices.WithStandardBits(choices, new[] { ((IReadOnlyList<Field>)fields, (IReadOnlyList<StandardObject>)objects) });

        Assert.Equal(new[] { "ReignOfChaos", "TheFrozenThrone", "Unlisted4", "Unlisted16" }, result["versionFlags"].Members.Select(m => m.Name));
        Assert.Equal(new[] { 1, 2, 4, 16 }, result["versionFlags"].Members.Select(m => m.Value));
        Assert.Equal("4", result["versionFlags"].Members[2].Text);
        Assert.Same(attack, result["attackType"]);
    }

    [Fact]
    public void the_enums_are_left_out_of_csharp_lua()
    {
        // Like the classes, the enums are inside #if !MOONWELL_LUA: they write no Lua, but a file of them still writes an empty namespace into every map.
        var files = GeneratedObjects.Make(MiniExport(), null, "3.0.1.24370");
        var lua = new CSharpParseOptions(LanguageVersion.CSharp13, preprocessorSymbols: new[] { "MOONWELL_LUA" });
        var dotnet = new CSharpParseOptions(LanguageVersion.CSharp13);

        var seenByLua = CSharpSyntaxTree.ParseText(files["Choices.g.cs"], lua).GetCompilationUnitRoot();
        var seenByDotnet = CSharpSyntaxTree.ParseText(files["Choices.g.cs"], dotnet).GetCompilationUnitRoot();

        Assert.Empty(seenByLua.Members);
        Assert.NotEmpty(seenByDotnet.Members);
        Assert.StartsWith("// Generated by Moonwell.Generator from the game's data, game 3.0.1.24370. Do not edit.\n\n#if !MOONWELL_LUA\n", files["Choices.g.cs"]);
    }

    [Fact]
    public void a_choice_property_holds_the_enum()
    {
        var files = GeneratedObjects.Make(MiniExport(), null, "3.0.1.24370");

        Assert.Contains(
            "        public global::Moonwell.AttackType AttackType\n"
            + "        {\n"
            + "            get { return Fields.Get<global::Moonwell.AttackType>(this, \"ua1t\"); }\n"
            + "            protected set { Fields.Put(\"ua1t\", value); }\n"
            + "        }\n",
            files["Kinds/StandardUnit.g.cs"]);
        Assert.Contains("public global::Moonwell.Objects.Levels<global::Moonwell.Objects.Targets> TargetsAllowed\n", files["Kinds/StandardAbility.g.cs"]);
        Assert.Contains("[\"ua1t\"] = typeof(global::Moonwell.AttackType),", files["Kinds/StandardUnit.g.cs"]);
        Assert.Contains("{\"id\":\"ua1t\",\"type\":\"choice\",\"perLevel\":false,\"pointer\":0,\"skin\":false,\"store\":\"string\",\"choice\":\"AttackType\"}", files["Values/unit.json"]);
    }
}
