using Moonwell.GameData;

namespace Moonwell.Generator.Tests;

public class StandardObjectsTests
{
    private static string MiniExportFolder()
    {
        return Path.GetFullPath(
            Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Fixtures", "MiniExport", "war3.w3mod"));
    }

    private static IReadOnlyList<StandardObject> Of(GameTables tables, Kind kind)
    {
        return StandardObjects.Of(tables, kind, Fields.Of(tables, kind));
    }

    private static IReadOnlyList<StandardObject> Of(Kind kind)
    {
        return Of(GameTables.OpenExport(MiniExportFolder()), kind);
    }

    private static StandardObject Find(IReadOnlyList<StandardObject> objects, string id)
    {
        return objects.Single(o => o.Id == id);
    }

    /// <summary>A copy of MiniExport in a temporary folder, for a test that changes a cell of it.</summary>
    private sealed class Copy : IDisposable
    {
        private readonly string _folder = Path.Combine(Path.GetTempPath(), "moonwell-" + Guid.NewGuid().ToString("N"));

        public Copy()
        {
            Directory.CreateDirectory(_folder);
            CopyFolder(MiniExportFolder(), _folder);
        }

        public GameTables Tables()
        {
            return GameTables.OpenExport(_folder);
        }

        /// <summary>Replaces the first <paramref name="find"/> after the first <paramref name="anchor"/> in a file.</summary>
        public void Replace(string relative, string anchor, string find, string replace)
        {
            var path = Path.Combine(_folder, relative);
            var text = File.ReadAllText(path);
            var start = text.IndexOf(anchor, StringComparison.Ordinal);
            Assert.True(start >= 0, $"{relative} has no '{anchor}'");
            var at = text.IndexOf(find, start, StringComparison.Ordinal);
            Assert.True(at >= 0, $"{relative} has no '{find}' after '{anchor}'");
            File.WriteAllText(path, text.Substring(0, at) + replace + text.Substring(at + find.Length));
        }

        /// <summary>Replaces the footman's <c>modelScale</c> lines (the file has :de, :hd and :sd ones) with these lines.</summary>
        public void SetFootmanScales(params string[] lines)
        {
            var path = Path.Combine(_folder, "units/unitskin.txt");
            var text = File.ReadAllText(path);
            var start = text.IndexOf("modelScale", text.IndexOf("[hfoo]", StringComparison.Ordinal), StringComparison.Ordinal);
            var end = text.IndexOf("legacyModelScale", start, StringComparison.Ordinal);
            Assert.True(start >= 0 && end > start, "the footman has no modelScale lines before legacyModelScale");
            File.WriteAllText(path, text.Substring(0, start) + string.Join("\r\n", lines) + "\r\n" + text.Substring(end));
        }

        public void Write(string relative, string text)
        {
            var path = Path.Combine(_folder, relative);
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            File.WriteAllText(path, text);
        }

        public string Read(string relative)
        {
            return File.ReadAllText(Path.Combine(_folder, relative));
        }

        public void Dispose()
        {
            Directory.Delete(_folder, true);
        }

        private static void CopyFolder(string from, string to)
        {
            foreach (var directory in Directory.GetDirectories(from))
            {
                var target = Path.Combine(to, Path.GetFileName(directory));
                Directory.CreateDirectory(target);
                CopyFolder(directory, target);
            }

            foreach (var file in Directory.GetFiles(from))
            {
                File.Copy(file, Path.Combine(to, Path.GetFileName(file)));
            }
        }
    }

    [Fact]
    public void paladin_values()
    {
        var paladin = Find(Of(Kind.Unit), "Hpal");

        Assert.Equal(Kind.Unit, paladin.Kind);
        Assert.Equal("100", paladin.Values["uhpm"]);
        Assert.Equal("Paladin", paladin.Values["unam"]);
        Assert.Equal("Paladin", paladin.Name);
        Assert.True(paladin.IsHero);
        Assert.Null(paladin.Code);
        Assert.False(Find(Of(Kind.Unit), "hfoo").IsHero);
    }

    [Fact]
    public void a_value_of_the_definitive_edition_graphics_is_the_games()
    {
        // The skin files key a value of each graphics mode (modelScale:sd, :de, :hd); the game, in Definitive Edition
        // graphics, scales a footman to 0.8 and a paladin to 0.935 (game-facts, seen on 3.0.1).
        var units = Of(Kind.Unit);

        Assert.Equal("0.8", Find(units, "hfoo").Values["usca"]);
        Assert.Equal("0.935", Find(units, "Hpal").Values["usca"]);
    }

    [Fact]
    public void a_value_of_the_definitive_edition_graphics_wins_over_the_plain_key()
    {
        // The peasant has modelScale=1 and modelScale:de=0.85, and the game scales it to 0.85.
        using var copy = new Copy();
        copy.Replace("units/unitskin.txt", "[hfoo]", "modelScale:de=0.8", "modelScale=1\nmodelScale:de=0.85");

        Assert.Equal("0.85", Find(Of(copy.Tables(), Kind.Unit), "hfoo").Values["usca"]);
    }

    [Fact]
    public void a_value_with_no_definitive_edition_one_is_the_reforged_one_not_the_classic_one()
    {
        // The game, in Definitive Edition graphics, scales an Assassin (hd 1.2, sd 1.3) to 1.2 and a Guardian Golem
        // (hd 1.0, sd 1.5) to 1.0: with no :de value it reads :hd, never :sd (game-facts, seen on 3.0.1).
        using var copy = new Copy();
        copy.SetFootmanScales("modelScale:hd=1.2", "modelScale:sd=1.3");
        Assert.Equal("1.2", Find(Of(copy.Tables(), Kind.Unit), "hfoo").Values["usca"]);

        copy.SetFootmanScales("modelScale:hd=1.0", "modelScale:sd=1.5");
        Assert.Equal("1.0", Find(Of(copy.Tables(), Kind.Unit), "hfoo").Values["usca"]);
    }

    [Fact]
    public void a_value_of_the_reforged_graphics_wins_over_the_plain_key_and_loses_to_the_definitive_edition_one()
    {
        // Plain 1 beside :hd 0.9 gives 0.9; with a :de 0.85 as well, 0.85.
        using var copy = new Copy();
        copy.SetFootmanScales("modelScale=1", "modelScale:hd=0.9", "modelScale:sd=1.3");
        Assert.Equal("0.9", Find(Of(copy.Tables(), Kind.Unit), "hfoo").Values["usca"]);

        copy.SetFootmanScales("modelScale=1", "modelScale:de=0.85", "modelScale:hd=0.9", "modelScale:sd=1.3");
        Assert.Equal("0.85", Find(Of(copy.Tables(), Kind.Unit), "hfoo").Values["usca"]);
    }

    [Fact]
    public void a_blank_reforged_value_with_no_definitive_edition_one_is_no_value()
    {
        // As with a blank :de value: the key has none, not the plain key's or the classic one.
        using var copy = new Copy();
        copy.SetFootmanScales("modelScale=1", "modelScale:hd=", "modelScale:sd=1.3");

        Assert.False(Find(Of(copy.Tables(), Kind.Unit), "hfoo").Values.ContainsKey("usca"));
    }

    [Fact]
    public void objects_are_ordered_by_id()
    {
        var ids = Of(Kind.Unit).Select(o => o.Id).ToList();

        Assert.Equal(new[] { "Hpal", "hfoo" }, ids);
        Assert.Equal(new[] { "AHhb", "AHtb" }, Of(Kind.Ability).Select(o => o.Id));
    }

    [Fact]
    public void storm_bolt_levels()
    {
        var abilities = Of(Kind.Ability);

        var stormBolt = Find(abilities, "AHtb");
        Assert.Equal("AHtb", stormBolt.Code);
        Assert.Equal("100", stormBolt.Values["Htb1:1"]);
        Assert.Equal("200", stormBolt.Values["Htb1:2"]);
        Assert.Equal("310", stormBolt.Values["Htb1:3"]);
        Assert.DoesNotContain("Htb1", stormBolt.Values.Keys);
        Assert.DoesNotContain(stormBolt.Values.Keys, key => key.StartsWith("Hhb1", StringComparison.Ordinal));
        var holyLight = Find(abilities, "AHhb");
        Assert.Equal("200", holyLight.Values["Hhb1:1"]);
        Assert.DoesNotContain(holyLight.Values.Keys, key => key.StartsWith("Htb1", StringComparison.Ordinal));
        Assert.Equal("3", stormBolt.Values["alev"]);
    }

    [Fact]
    public void a_level_cell_is_the_files_text_and_a_dash_is_no_value()
    {
        var stormBolt = Find(Of(Kind.Ability), "AHtb");

        Assert.Equal("9", stormBolt.Values["acdn:1"]);
        Assert.Equal("75", stormBolt.Values["amcs:3"]);
        Assert.Equal("BPSE", stormBolt.Values["abuf:2"]);
        Assert.DoesNotContain("aare:1", stormBolt.Values.Keys);
        Assert.DoesNotContain("aare:2", stormBolt.Values.Keys);
    }

    [Fact]
    public void an_abilitys_columns_past_its_level_count_are_values_and_a_level_past_the_fourth_takes_the_one_before()
    {
        // Storm Bolt has 3 levels; its table holds DataA4 325, Cool4 9 and Cost4 75, and "-" in the columns of levels 5 and 6.
        // The game gives a Storm Bolt of 6 levels 325 damage, 9 cooldown and 75 mana at levels 4 to 6 (game-facts, 3.0.1).
        var stormBolt = Find(Of(Kind.Ability), "AHtb");

        Assert.Equal("325", stormBolt.Values["Htb1:4"]);
        Assert.Equal("325", stormBolt.Values["Htb1:5"]);
        Assert.Equal("325", stormBolt.Values["Htb1:6"]);
        Assert.Equal("75", stormBolt.Values["amcs:6"]);
        Assert.Equal("9", stormBolt.Values["acdn:5"]);
        Assert.DoesNotContain("Htb1:7", stormBolt.Values.Keys);

        // Cast4 is "-": level 4 has no value, and so neither do 5 and 6.
        Assert.DoesNotContain(stormBolt.Values.Keys, key => key is "acas:4" or "acas:5" or "acas:6");

        // A text list per level is not a column of each level: it has values for the levels only.
        Assert.DoesNotContain("atp1:4", stormBolt.Values.Keys);
    }

    [Fact]
    public void per_level_text_has_one_entry_per_level()
    {
        var stormBolt = Find(Of(Kind.Ability), "AHtb");

        Assert.Equal("Storm Bolt - [|cffffcc00Level 1|r]", stormBolt.Values["atp1:1"]);
        Assert.Equal("Storm Bolt - [|cffffcc00Level 3|r]", stormBolt.Values["atp1:3"]);
        Assert.DoesNotContain("atp1:4", stormBolt.Values.Keys);
        Assert.Equal(
            "Throws a magical hammer at a target enemy unit, dealing <AHtb,DataA2> damage and stunning the target for <AHtb,Dur2> seconds.",
            stormBolt.Values["aub1:2"]);
    }

    [Fact]
    public void a_quoted_entry_with_commas_is_one_entry()
    {
        using var copy = new Copy();
        copy.Replace(
            "_locales/enus.w3mod/units/humanabilitystrings.txt",
            "[AHtb]",
            "Tip=Storm Bolt - [|cffffcc00Level 1|r],Storm Bolt - [|cffffcc00Level 2|r],Storm Bolt - [|cffffcc00Level 3|r]",
            "Tip=\"Storm Bolt, again\"");

        var stormBolt = Find(Of(copy.Tables(), Kind.Ability), "AHtb");

        Assert.Equal("Storm Bolt, again", stormBolt.Values["atp1:1"]);
        Assert.DoesNotContain("atp1:2", stormBolt.Values.Keys);
    }

    [Fact]
    public void a_text_field_with_an_index_alone_on_its_key_is_that_entry_of_the_list()
    {
        // Spell Immunity's standard name is "Spell Immunity, Spell Immunity"; the metadata gives anam index 0.
        using var copy = new Copy();
        copy.Replace("_locales/enus.w3mod/units/humanabilitystrings.txt", "[AHtb]", "Name=Storm Bolt", "Name=Storm Bolt, Storm Bolt Again");
        copy.Replace("_locales/enus.w3mod/units/humanabilitystrings.txt", "[AHhb]", "Name=Holy Light", "Name=\"Holy Light, Again\"");

        var abilities = Of(copy.Tables(), Kind.Ability);

        Assert.Equal("Storm Bolt", Find(abilities, "AHtb").Values["anam"]);
        Assert.Equal("Storm Bolt", Find(abilities, "AHtb").Name);
        Assert.Equal("Holy Light, Again", Find(abilities, "AHhb").Values["anam"]);
    }

    [Fact]
    public void an_unquoted_list_splits_at_every_comma()
    {
        using var copy = new Copy();
        copy.Replace(
            "_locales/enus.w3mod/units/humanabilitystrings.txt",
            "[AHtb]",
            "Tip=Storm Bolt - [|cffffcc00Level 1|r],Storm Bolt - [|cffffcc00Level 2|r],Storm Bolt - [|cffffcc00Level 3|r]",
            "Tip=Storm Bolt, again");

        var stormBolt = Find(Of(copy.Tables(), Kind.Ability), "AHtb");

        Assert.Equal("Storm Bolt", stormBolt.Values["atp1:1"]);
        Assert.Equal("again", stormBolt.Values["atp1:2"]);
        Assert.DoesNotContain("atp1:3", stormBolt.Values.Keys);
    }

    [Fact]
    public void an_upgrade_level_n_requires_the_key_n_minus_1()
    {
        var upgrade = Find(Of(Kind.Upgrade), "Rhme");

        Assert.Equal("3", upgrade.Values["glvl"]);
        Assert.Equal("Iron Forged Swords", upgrade.Values["gnam:1"]);
        Assert.Equal("Mithril Forged Swords", upgrade.Values["gnam:3"]);
        Assert.Equal("Iron Forged Swords", upgrade.Name);
        Assert.Equal("S", upgrade.Values["ghk1:2"]);
        Assert.DoesNotContain("greq:1", upgrade.Values.Keys);
        Assert.Equal("hkee", upgrade.Values["greq:2"]);
        Assert.Equal("hcas", upgrade.Values["greq:3"]);
        Assert.DoesNotContain("greq:4", upgrade.Values.Keys);
    }

    [Fact]
    public void a_field_that_shares_its_key_is_one_element_of_the_list()
    {
        var paladin = Find(Of(Kind.Unit), "Hpal");
        var stormBolt = Find(Of(Kind.Ability), "AHtb");

        Assert.Equal("2", paladin.Values["ubpx"]);
        Assert.Equal("2", paladin.Values["ubpy"]);
        Assert.Equal("0", stormBolt.Values["abpx"]);
        Assert.Equal("2", stormBolt.Values["abpy"]);
    }

    [Fact]
    public void set_diff_only()
    {
        var footman = Find(Of(Kind.Unit), "hfoo");

        Assert.Equal("420", footman.Values["uhpm"]);
        var melee = footman.Sets["melee_v0"];
        Assert.Equal(new[] { "uhpm" }, melee.Keys);
        Assert.Equal("450", melee["uhpm"]);
        Assert.Empty(footman.Sets["custom_v0"]);
        Assert.Empty(footman.MissingFrom);
    }

    [Fact]
    public void an_object_a_sets_tables_leave_out_is_missing_from_it()
    {
        var paladin = Find(Of(Kind.Unit), "Hpal");

        Assert.Equal(new[] { "melee_v0" }, paladin.MissingFrom);
        Assert.False(paladin.Sets.ContainsKey("melee_v0"));
        Assert.Equal("750", paladin.Sets["custom_v0"]["uhpm"]);
        Assert.Equal(new[] { "uhpm" }, paladin.Sets["custom_v0"].Keys);
    }

    [Fact]
    public void a_kind_no_set_replaces_has_empty_sets()
    {
        var doodads = Of(Kind.Doodad);

        Assert.NotEmpty(doodads);
        Assert.All(doodads, o => Assert.Empty(o.MissingFrom));
        Assert.All(doodads, o => Assert.All(o.Sets.Values, set => Assert.Empty(set)));
    }

    [Fact]
    public void a_value_a_set_drops_is_the_empty_string()
    {
        using var copy = new Copy();
        copy.Replace("_balance/melee_v0.w3mod/units/unitbalance.slk", "C;X23;K450", "C;X23;K450\r\n", "");

        var footman = Find(Of(copy.Tables(), Kind.Unit), "hfoo");

        Assert.Equal("", footman.Sets["melee_v0"]["uhpm"]);
    }

    [Fact]
    public void an_object_only_a_set_has_takes_that_sets_values()
    {
        using var copy = new Copy();
        copy.Write("_balance/melee_v0.w3mod/units/unitdata.slk", copy.Read("units/unitdata.slk").Replace("\"hfoo\"", "\"hfox\""));
        copy.Replace("_balance/melee_v0.w3mod/units/unitbalance.slk", "C;X1;Y2;", "\"hfoo\"", "\"hfox\"");

        var units = Of(copy.Tables(), Kind.Unit);

        var added = Find(units, "hfox");
        Assert.Equal("450", added.Values["uhpm"]);
        Assert.Equal(new[] { "base", "custom_v0" }, added.MissingFrom);
        Assert.Empty(added.Sets["melee_v0"]);
        Assert.Equal(new[] { "melee_v0" }, Find(units, "hfoo").MissingFrom);
    }

    [Fact]
    public void race_and_campaign()
    {
        using var copy = new Copy();
        copy.Replace("units/unitskin.txt", "[Hpal]", "campaign=0", "campaign=1");

        var units = Of(copy.Tables(), Kind.Unit);

        var paladin = Find(units, "Hpal");
        Assert.Equal("human", paladin.Race);
        Assert.True(paladin.Campaign);
        var footman = Find(units, "hfoo");
        Assert.Equal("human", footman.Race);
        Assert.False(footman.Campaign);
        Assert.Equal("human", Find(Of(Kind.Ability), "AHtb").Race);
        Assert.Equal("human", Find(Of(Kind.Upgrade), "Rhme").Race);
        Assert.False(Find(Of(Kind.Ability), "AHtb").Campaign);
    }

    [Fact]
    public void a_unit_has_the_fields_of_its_use_flags()
    {
        using var copy = new Copy();
        copy.Replace("units/unitbalance.slk", "C;X1;Y3;K\"hfoo\"", "C;X53;K0", "C;X53;K1");

        var units = Of(copy.Tables(), Kind.Unit);

        var paladin = Find(units, "Hpal");
        Assert.Contains("ustr", paladin.Values.Keys);
        Assert.Contains("ulev", paladin.Values.Keys);
        var building = Find(units, "hfoo");
        Assert.Contains("uhpm", building.Values.Keys);
        Assert.DoesNotContain("ustr", building.Values.Keys);
        Assert.DoesNotContain("ulev", building.Values.Keys);
        var plain = Find(Of(Kind.Unit), "hfoo");
        Assert.DoesNotContain("ustr", plain.Values.Keys);
        Assert.Equal("2", plain.Values["ulev"]);
    }

    [Fact]
    public void an_item_has_the_item_fields()
    {
        var item = Find(Of(Kind.Item), "ratf");

        Assert.Equal("800", item.Values["igol"]);
        Assert.Equal("AItf", item.Values["iabi"]);
        Assert.False(item.IsHero);
        Assert.Null(item.Code);
    }

    [Fact]
    public void a_common_field_is_left_out_where_the_metadata_excludes_it()
    {
        using var copy = new Copy();
        copy.Replace("units/abilitydata.slk", "C;X1;Y3;K\"AHhb\"", "C;X2;K\"AHhb\"", "C;X2;K\"Arpb\"");

        var abilities = Of(copy.Tables(), Kind.Ability);

        var holyLight = Find(abilities, "AHhb");
        Assert.Equal("Arpb", holyLight.Code);
        Assert.DoesNotContain("acas:1", holyLight.Values.Keys);
        Assert.Contains("acdn:1", holyLight.Values.Keys);

        // Hhb1 lists AHhb, which is the ability's id: it applies whatever the code.
        Assert.Contains("Hhb1:1", holyLight.Values.Keys);
        Assert.Equal("0", Find(abilities, "AHtb").Values["acas:1"]);
    }

    [Fact]
    public void an_object_has_its_editor_suffix_trimmed()
    {
        using var copy = new Copy();
        copy.Replace("_locales/enus.w3mod/units/humanunitstrings.txt", "[hfoo]", "Name=Footman", "Name=Footman\r\nEditorSuffix= (Campaign) ");

        var units = Of(copy.Tables(), Kind.Unit);

        Assert.Equal("(Campaign)", Find(units, "hfoo").EditorSuffix);
        Assert.Equal("", Find(units, "Hpal").EditorSuffix);
    }

    [Fact]
    public void an_editor_suffix_marked_as_the_first_entry_of_a_list_is_found()
    {
        // The game's metadata gives unsf and ansf index 0, so their column is EditorSuffix[0].
        using var copy = new Copy();
        copy.Replace("units/unitmetadata.slk", "K\"unsf\"", "C;X4;K-1", "C;X4;K0");
        copy.Replace("_locales/enus.w3mod/units/humanunitstrings.txt", "[hfoo]", "Name=Footman", "Name=Footman\r\nEditorSuffix=(Campaign), (Other)");

        Assert.Equal("(Campaign)", Find(Of(copy.Tables(), Kind.Unit), "hfoo").EditorSuffix);
    }

    [Fact]
    public void an_upgrade_has_the_editor_suffix_of_its_first_level()
    {
        using var copy = new Copy();
        copy.Replace("_locales/enus.w3mod/units/humanupgradestrings.txt", "[Rhme]", "Name=", "EditorSuffix=(First),(Second)\r\nName=");

        Assert.Equal("(First)", Find(Of(copy.Tables(), Kind.Upgrade), "Rhme").EditorSuffix);
    }

    [Fact]
    public void a_field_listed_by_the_abilitys_own_id_applies_and_takes_its_column_from_one_listed_by_the_code()
    {
        // As Cargo Capacity lists Sch2 to Sch5, not their code Amtc; and as AIls has Idps (by id), not Lsh1 (by code), on DataA.
        using var copy = new Copy();
        copy.Replace("units/abilitydata.slk", "C;X1;Y3;K\"AHhb\"", "C;X2;K\"AHhb\"", "C;X2;K\"AHtb\"");

        var holyLight = Find(Of(copy.Tables(), Kind.Ability), "AHhb");

        Assert.Equal("AHtb", holyLight.Code);
        Assert.Equal("200", holyLight.Values["Hhb1:1"]);
        Assert.DoesNotContain(holyLight.Values.Keys, key => key.StartsWith("Htb1", StringComparison.Ordinal));
    }

    [Fact]
    public void a_doodad_has_a_name_and_no_race()
    {
        var doodad = Of(Kind.Doodad).First();

        Assert.NotEmpty(doodad.Name);
        Assert.Equal("Lilypad", doodad.Name);
        Assert.Empty(doodad.Race);
        Assert.False(doodad.Campaign);
    }

    [Fact]
    public void a_world_editor_string_key_as_a_name_is_resolved_to_the_label()
    {
        var tables = GameTables.OpenExport(MiniExportFolder());
        var doodad = Find(Of(tables, Kind.Doodad), "ZPlp");
        var destructable = Find(Of(tables, Kind.Destructable), "LTlt");

        Assert.Equal("Lilypad", doodad.Name);
        Assert.Equal("Summer Tree Wall", destructable.Name);
    }

    [Fact]
    public void the_values_keep_the_raw_key()
    {
        var tables = GameTables.OpenExport(MiniExportFolder());

        Assert.Equal("WESTRING_DOOD_LPLP", Find(Of(tables, Kind.Doodad), "ZPlp").Values["dnam"]);
        Assert.Equal("WESTRING_DEST_SUMMER_TREE_WALL", Find(Of(tables, Kind.Destructable), "LTlt").Values["bnam"]);
    }

    [Fact]
    public void a_world_editor_string_key_without_a_label_is_no_name()
    {
        using var copy = new Copy();
        copy.Replace("doodads/doodadskins.txt", "[ZPlp]", "Name=WESTRING_DOOD_LPLP", "Name=WESTRING_DOOD_NO_SUCH_LABEL");

        var doodad = Find(Of(copy.Tables(), Kind.Doodad), "ZPlp");
        Assert.Equal("", doodad.Name);
        Assert.Equal("WESTRING_DOOD_NO_SUCH_LABEL", doodad.Values["dnam"]);
    }

    [Fact]
    public void a_key_that_differs_from_its_labels_key_in_case_is_resolved()
    {
        using var copy = new Copy();
        copy.Replace("_locales/enus.w3mod/ui/worldeditgamestrings.txt", "[WorldEditStrings]", "WESTRING_DEST_SUMMER_TREE_WALL=", "WESTRING_Dest_Summer_Tree_Wall=");

        var destructable = Find(Of(copy.Tables(), Kind.Destructable), "LTlt");
        Assert.Equal("Summer Tree Wall", destructable.Name);
        Assert.Equal("WESTRING_DEST_SUMMER_TREE_WALL", destructable.Values["bnam"]);
    }
}
