using Moonwell.Generator.Objects;

namespace Moonwell.Generator.Tests;

public class ExportTablesTests
{
    private static string MiniExport()
    {
        return Path.GetFullPath(
            Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Fixtures", "MiniExport", "war3.w3mod"));
    }

    private static ObjectRows Find(IReadOnlyList<ObjectRows> objects, string id)
    {
        return objects.Single(o => o.Id == id);
    }

    [Fact]
    public void units_merge_tables_and_text()
    {
        var units = ExportTables.Read(MiniExport()).Objects(Kind.Unit, null);

        Assert.Equal(new[] { "Hpal", "hfoo" }, units.Select(u => u.Id));
        var paladin = Find(units, "Hpal").Values;
        Assert.Equal("100", paladin["HP"]);
        Assert.Equal("a1", paladin["sortBalance"]);
        Assert.Equal("HeroPaladin", paladin["comment(s)"]);
        Assert.Equal("Paladin", paladin["Name"]);
        Assert.Equal("2,2", paladin["Buttonpos"]);
        Assert.Equal(@"ReplaceableTextures\CommandButtons\BTNHeroPaladin.blp", paladin["Art"]);
        Assert.Equal("Hpal", paladin["skinnableID"]);
        Assert.StartsWith("Warrior Hero, exceptional at defense", paladin["Ubertip"]);
        Assert.EndsWith("|r", paladin["Ubertip"]);
        Assert.Equal("Footman", Find(units, "hfoo").Values["Name"]);
    }

    [Fact]
    public void values_are_the_files_text()
    {
        var units = ExportTables.Read(MiniExport()).Objects(Kind.Unit, null);

        var paladin = Find(units, "Hpal").Values;
        var footman = Find(units, "hfoo").Values;
        Assert.Equal("0.01", paladin["regenMana"]);
        Assert.Equal(" - ", footman["regenMana"]);
        Assert.Equal("1", footman["stockInitial"]);
        Assert.Equal("1", footman["InBeta"]);
    }

    [Fact]
    public void a_quoted_list_keeps_its_quotes()
    {
        var upgrades = ExportTables.Read(MiniExport()).Objects(Kind.Upgrade, null);

        var values = Find(upgrades, "Rhme").Values;
        Assert.StartsWith("\"Increases the attack damage of Militia", values["Ubertip"]);
        Assert.Contains("\",\"Further increases", values["Ubertip"]);
        Assert.Equal("S,S,S", values["Hotkey"]);
        Assert.Equal("Iron Forged Swords,Steel Forged Swords,Mithril Forged Swords", values["Name"]);
    }

    [Fact]
    public void a_value_wrapped_in_quotes_is_named()
    {
        var tables = ExportTables.Read(MiniExport());

        var paladin = Find(tables.Objects(Kind.Unit, null), "Hpal");
        Assert.Contains("Ubertip", paladin.Wrapped!);
        Assert.DoesNotContain("Name", paladin.Wrapped!);
        var upgradeWrapped = Find(tables.Objects(Kind.Upgrade, null), "Rhme").Wrapped;
        Assert.True(upgradeWrapped == null || !upgradeWrapped.Contains("Ubertip"));
    }

    [Theory]
    [InlineData(Kind.Ability, "AHtb", "Buttonpos", "0,2")]
    [InlineData(Kind.Ability, "AHhb", "alias", "AHhb")]
    [InlineData(Kind.Buff, "BHbd", "alias", "BHbd")]
    [InlineData(Kind.Item, "ratf", "skinType", "item")]
    [InlineData(Kind.Item, "ratf", "itemID", "ratf")]
    [InlineData(Kind.Destructable, "LTlt", "skinType", "destructable")]
    [InlineData(Kind.Doodad, "ZPlp", "skinType", "doodad")]
    [InlineData(Kind.Doodad, "ZPlp", "doodID", "ZPlp")]
    [InlineData(Kind.Upgrade, "Rhme", "Requires1", "hkee")]
    public void every_kind_reads_its_tables_and_text(Kind kind, string id, string key, string expected)
    {
        var objects = ExportTables.Read(MiniExport()).Objects(kind, null);

        Assert.Equal(expected, Find(objects, id).Values.GetValueOrDefault(key));
    }

    [Fact]
    public void text_of_an_id_no_table_holds_is_dropped()
    {
        var tables = ExportTables.Read(MiniExport());

        Assert.DoesNotContain(tables.Objects(Kind.Ability, null), o => o.Id == "BHbd");
        Assert.DoesNotContain(tables.Objects(Kind.Unit, null), o => o.Id == "Y425");
    }

    [Fact]
    public void a_row_only_a_later_table_holds_is_dropped()
    {
        // unitweapons.slk of the fixture, as of the real export, has a row for nrmf that unitdata.slk lacks.
        var units = ExportTables.Read(MiniExport()).Objects(Kind.Unit, null);

        Assert.DoesNotContain(units, u => u.Id == "nrmf");
    }

    [Fact]
    public void balance_set_replaces_file()
    {
        var tables = ExportTables.Read(MiniExport());

        var baseFootman = Find(tables.Objects(Kind.Unit, null), "hfoo").Values;
        var meleeUnits = tables.Objects(Kind.Unit, "melee_v0");
        var meleeFootman = Find(meleeUnits, "hfoo").Values;
        Assert.Equal("420", baseFootman["HP"]);
        Assert.Equal("450", meleeFootman["HP"]);
        Assert.Equal("Footman", meleeFootman["Name"]);
        Assert.DoesNotContain("sortBalance", Find(meleeUnits, "Hpal").Values.Keys);
        Assert.Equal("Paladin", Find(meleeUnits, "Hpal").Values["Name"]);
    }

    [Fact]
    public void a_file_the_set_lacks_comes_from_the_base()
    {
        var tables = ExportTables.Read(MiniExport());

        var meleeAbilities = tables.Objects(Kind.Ability, "melee_v0");

        Assert.Equal(tables.Objects(Kind.Ability, null).Select(o => o.Id), meleeAbilities.Select(o => o.Id));
    }

    [Fact]
    public void an_unknown_set_is_refused()
    {
        var tables = ExportTables.Read(MiniExport());

        var error = Assert.Throws<ArgumentException>(() => tables.Objects(Kind.Unit, "custom_v9"));

        Assert.Contains("custom_v9", error.Message);
    }

    [Fact]
    public void a_set_missing_from_the_export_is_named()
    {
        var tables = ExportTables.Read(MiniExport());

        var error = Assert.Throws<DirectoryNotFoundException>(() => tables.Objects(Kind.Unit, "custom_v1"));

        Assert.Contains("_balance/custom_v1.w3mod", error.Message);
    }

    [Fact]
    public void the_sets_an_export_holds()
    {
        var tables = ExportTables.Read(MiniExport());

        Assert.Equal(new[] { "melee_v0", "custom_v0" }, tables.Sets);
    }

    [Fact]
    public void present_ids_need_a_row_in_every_table_the_set_replaces()
    {
        var tables = ExportTables.Read(MiniExport());

        Assert.Equal(new[] { "hfoo" }, tables.Present(Kind.Unit, "melee_v0").OrderBy(id => id, StringComparer.Ordinal));
        Assert.Equal(new[] { "Hpal", "hfoo" }, tables.Present(Kind.Unit, "custom_v0").OrderBy(id => id, StringComparer.Ordinal));
    }

    [Fact]
    public void a_kind_no_table_of_the_set_replaces_is_present_whole()
    {
        var tables = ExportTables.Read(MiniExport());

        Assert.Equal(
            tables.Objects(Kind.Doodad, null).Select(o => o.Id).OrderBy(id => id, StringComparer.Ordinal),
            tables.Present(Kind.Doodad, "melee_v0").OrderBy(id => id, StringComparer.Ordinal));
    }

    [Fact]
    public void meta_rows_by_column()
    {
        var meta = ExportTables.Read(MiniExport()).Meta(Kind.Unit);

        var row = meta.Single(r => r.Id == "uhpm");
        Assert.Equal("HP", row.Columns["field"]);
        Assert.Equal("UnitBalance", row.Columns["slk"]);
        Assert.Equal("WESTRING_UEVAL_UHPM", row.Columns["displayName"]);
        Assert.Equal("int", row.Columns["type"]);
        Assert.Equal("0", row.Columns["netsafe"]);
        Assert.Equal("1", row.Columns["useUnit"]);
    }

    [Fact]
    public void meta_of_each_kind_comes_from_its_table()
    {
        var tables = ExportTables.Read(MiniExport());

        Assert.Equal("1", tables.Meta(Kind.Item).Single(r => r.Id == "iabi").Columns["useItem"]);
        var data = tables.Meta(Kind.Ability).Single(r => r.Id == "Htb1").Columns;
        Assert.Equal("Data", data["field"]);
        Assert.Equal("4", data["repeat"]);
        Assert.Equal("1", data["data"]);
        Assert.Contains(tables.Meta(Kind.Buff), r => r.Id == "fnam");
        Assert.Contains(tables.Meta(Kind.Upgrade), r => r.Id == "gnam");
        Assert.Contains(tables.Meta(Kind.Destructable), r => r.Id == "bnam");
        Assert.Contains(tables.Meta(Kind.Doodad), r => r.Id == "dnam");
    }

    [Fact]
    public void labels_from_locale()
    {
        var tables = ExportTables.Read(MiniExport());

        Assert.Equal("Hit Points Maximum (Base)", tables.Label("WESTRING_UEVAL_UHPM"));
        Assert.Equal("Abilities", tables.Label("WESTRING_UEVAL_IABI"));
        Assert.Equal("WESTRING_NO_SUCH_LABEL", tables.Label("WESTRING_NO_SUCH_LABEL"));
    }

    [Fact]
    public void missing_file_names_it()
    {
        using var copy = new TempExport(MiniExport());
        var tables = ExportTables.Read(copy.Folder);
        File.Delete(Path.Combine(copy.Folder, "units", "unitbalance.slk"));

        var error = Assert.Throws<FileNotFoundException>(() => tables.Objects(Kind.Unit, null));

        Assert.Contains("units/unitbalance.slk", error.Message);
        Assert.Contains(copy.Folder, error.Message);
    }

    [Fact]
    public void missing_labels_file_names_it()
    {
        using var copy = new TempExport(MiniExport());
        File.Delete(Path.Combine(copy.Folder, "_locales", "enus.w3mod", "ui", "worldeditstrings.txt"));

        var error = Assert.Throws<FileNotFoundException>(() => ExportTables.Read(copy.Folder));

        Assert.Contains("_locales/enus.w3mod/ui/worldeditstrings.txt", error.Message);
    }

    [Fact]
    public void labels_from_the_game_file_too_the_editor_file_wins_and_case_does_not_matter()
    {
        var tables = ExportTables.Read(MiniExport());

        Assert.Equal("Summer Tree Wall", tables.Label("WESTRING_DEST_SUMMER_TREE_WALL"));
        Assert.Equal("Lilypad", tables.Label("WESTRING_DOOD_LPLP"));
        Assert.Equal("Lilypad", tables.Label("WESTRING_DOOD_lplp"));
    }

    [Fact]
    public void missing_game_labels_file_names_it()
    {
        using var copy = new TempExport(MiniExport());
        File.Delete(Path.Combine(copy.Folder, "_locales", "enus.w3mod", "ui", "worldeditgamestrings.txt"));

        var error = Assert.Throws<FileNotFoundException>(() => ExportTables.Read(copy.Folder));

        Assert.Contains("_locales/enus.w3mod/ui/worldeditgamestrings.txt", error.Message);
    }

    [Fact]
    public void missing_text_files_are_named_by_their_pattern()
    {
        using var copy = new TempExport(MiniExport());
        var tables = ExportTables.Read(copy.Folder);
        File.Delete(Path.Combine(copy.Folder, "units", "humanunitfunc.txt"));

        var error = Assert.Throws<FileNotFoundException>(() => tables.Objects(Kind.Unit, null));

        Assert.Contains("units/*unitfunc.txt", error.Message);
    }

    [Fact]
    public void path_case_does_not_matter()
    {
        using var copy = new TempExport(MiniExport());
        Directory.Move(Path.Combine(copy.Folder, "units"), Path.Combine(copy.Folder, "Units"));
        File.Move(
            Path.Combine(copy.Folder, "Units", "unitbalance.slk"),
            Path.Combine(copy.Folder, "Units", "UnitBalance.SLK"));

        var units = ExportTables.Read(copy.Folder).Objects(Kind.Unit, null);

        Assert.Equal("100", Find(units, "Hpal").Values["HP"]);
    }

    [Fact]
    public void a_semicolon_and_an_escaped_quote_in_a_cell_survive()
    {
        using var copy = new TempExport(MiniExport());
        File.WriteAllText(
            Path.Combine(copy.Folder, "units", "itemdata.slk"),
            "ID;PWXL;N;E\r\nB;X3;Y2;D0\r\nC;X1;Y1;K\"itemID\"\r\nC;X2;K\"comment\"\r\nC;X3;K\"level\"\r\n"
            + "C;X1;Y2;K\"ratf\"\r\nC;X2;K\"a; b \"\"c\"\"\"\r\nC;X3;K2.5\r\nE\r\n");

        var item = Find(ExportTables.Read(copy.Folder).Objects(Kind.Item, null), "ratf").Values;

        Assert.Equal("a; b \"c\"", item["comment"]);
        Assert.Equal("2.5", item["level"]);
    }

    private sealed class TempExport : IDisposable
    {
        public TempExport(string source)
        {
            Folder = Path.Combine(Path.GetTempPath(), "moonwell-export-" + Guid.NewGuid().ToString("N"), "war3.w3mod");
            Copy(source, Folder);
        }

        public string Folder { get; }

        public void Dispose()
        {
            Directory.Delete(Path.GetDirectoryName(Folder)!, true);
        }

        private static void Copy(string from, string to)
        {
            Directory.CreateDirectory(to);
            foreach (var file in Directory.GetFiles(from))
            {
                File.Copy(file, Path.Combine(to, Path.GetFileName(file)));
            }

            foreach (var directory in Directory.GetDirectories(from))
            {
                Copy(directory, Path.Combine(to, Path.GetFileName(directory)));
            }
        }
    }
}
