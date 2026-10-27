using Moonwell.GameData;
using Xunit.Abstractions;

namespace Moonwell.Generator.Tests;

/// <summary>
/// Reads the Warcraft III install on this computer (MOONWELL_WAR3, else the usual folder); without one the tests that
/// need it pass without checking anything, and say so.
/// </summary>
public sealed class InstallTests : IClassFixture<InstallTests.Install>
{
    private readonly Install _install;

    public InstallTests(Install install, ITestOutputHelper output)
    {
        _install = install;
        if (install.Files == null)
        {
            output.WriteLine(install.Reason);
        }
    }

    [Theory]
    [InlineData("units/unitdata.slk", "War3.w3mod:units/unitdata.slk")]
    [InlineData("_balance/custom_v1.w3mod/units/unitdata.slk", "War3.w3mod:_balance/custom_v1.w3mod:units/unitdata.slk")]
    [InlineData("_locales/enus.w3mod/ui/worldeditstrings.txt", "War3.w3mod:_locales/enus.w3mod:ui/worldeditstrings.txt")]
    [InlineData("_balance/melee_v0.w3mod", "War3.w3mod:_balance/melee_v0.w3mod")]
    public void an_export_path_is_the_storage_path_with_each_module_ended_by_a_colon(string export, string storage)
    {
        Assert.Equal(storage, StorageFiles.StoragePath(export));
    }

    [Fact]
    public void paths_are_written_the_ways_object_data_writes_them()
    {
        Assert.Equal("units/human/Peasant/Peasant", GameFiles.Normalized(@" \units\human\Peasant\Peasant "));
        Assert.Equal("Textures/x.dds", GameFiles.Normalized("War3.w3mod:Textures/x.dds"));
    }

    [Fact]
    public void an_export_has_no_build_and_is_in_english()
    {
        using var tables = GameTables.OpenExport(Path.GetFullPath(
            Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Fixtures", "MiniExport", "war3.w3mod")));

        Assert.Null(tables.Build);
        Assert.Equal("enUS", tables.Locale);
    }

    [Fact]
    public void the_install_has_the_tables_an_export_has()
    {
        if (_install.Files is not { } files)
        {
            return;
        }

        using var tables = GameTables.OpenInstall(files);

        Assert.Equal(new[] { "melee_v0", "custom_v0", "custom_v1" }, tables.Sets);
        Assert.Equal("Footman", tables.Objects(Kind.Unit, null).Single(unit => unit.Id == "hfoo").Values["Name"]);
        Assert.Equal("Holy Light", tables.Objects(Kind.Ability, "custom_v1").Single(ability => ability.Id == "AHhb").Values["Name"]);
        Assert.Contains("hfoo", tables.Present(Kind.Unit, "custom_v1"));
        Assert.NotEmpty(tables.Meta(Kind.Unit));
        Assert.Contains("StrHitPointBonus", tables.Misc(null).Keys);
        Assert.NotNull(tables.Choices("attackType"));
        Assert.NotEqual("WESTRING_UE_TARGETTYPE_AIR", tables.Label("WESTRING_UE_TARGETTYPE_AIR"));
        Assert.Equal(files.Build, tables.Build);
        Assert.Equal(files.Build.Locale, tables.Locale);
    }

    [Fact]
    public void a_locale_whose_files_are_not_installed_is_an_error_that_names_it()
    {
        if (_install.Files is not { } files || files.Build.Locale == "deDE")
        {
            return;
        }

        var error = Assert.Throws<FileNotFoundException>(() => GameTables.OpenInstall(files, "deDE"));

        Assert.Contains("deDE", error.Message);
    }

    [Fact]
    public void the_install_names_its_build()
    {
        if (_install.Files is not { } files)
        {
            return;
        }

        Assert.Equal(GameInstall.Build(_install.Folder!), files.Build);
        Assert.Matches(@"^\d+\.\d+\.\d+\.\d+$", files.Build.Version);
    }

    [Fact]
    public void a_unit_icon_and_model_are_found_as_the_game_stores_them()
    {
        if (_install.Files is not { } files)
        {
            return;
        }

        Assert.EndsWith(".dds", files.Find(@"ReplaceableTextures\CommandButtons\BTNPeasant.blp"), StringComparison.OrdinalIgnoreCase);
        Assert.EndsWith(".mdx", files.Find(@"units\human\Peasant\Peasant"), StringComparison.OrdinalIgnoreCase);
        Assert.EndsWith(".mdx", files.Find(@"units\human\Peasant\Peasant.mdl"), StringComparison.OrdinalIgnoreCase);
        Assert.Null(files.Find(@"units\human\NoSuchUnit\NoSuchUnit"));

        // Each graphics mode reads its own tier first: the Definitive Edition's Peasant icon is its own.
        Assert.StartsWith(
            "War3.w3mod:_DE.w3mod:",
            files.Find(@"ReplaceableTextures\CommandButtons\BTNPeasant.blp", "de"),
            StringComparison.OrdinalIgnoreCase);
        Assert.StartsWith(
            "War3.w3mod:ReplaceableTextures",
            files.Find(@"ReplaceableTextures\CommandButtons\BTNPeasant.blp", "sd"),
            StringComparison.OrdinalIgnoreCase);
        Assert.Contains(
            files.List("ReplaceableTextures/CommandButtons/", "de", ".dds", ".blp"),
            icon => icon.Contains("BTNPriestV1", StringComparison.OrdinalIgnoreCase));
        Assert.DoesNotContain(
            files.List("ReplaceableTextures/CommandButtons/", "sd", ".dds", ".blp"),
            icon => icon.Contains("BTNPriestV1", StringComparison.OrdinalIgnoreCase));

        // Only the Reforged tiers hold the icon the data gives the Priest.
        Assert.Contains(
            "_HD.w3mod:",
            files.Find(@"ReplaceableTextures\CommandButtons\BTNPriestV1.blp"),
            StringComparison.OrdinalIgnoreCase);

        var model = files.Read(@"units\human\Peasant\Peasant")!;
        Assert.Equal("MDLX"u8.ToArray(), model[..4]);
    }

    [Fact]
    public void the_icons_and_models_can_be_listed()
    {
        if (_install.Files is not { } files)
        {
            return;
        }

        var icons = files.List("ReplaceableTextures/CommandButtons/", "sd", ".dds", ".blp");
        Assert.True(icons.Count > 1000, $"{icons.Count} icons");
        Assert.Contains(icons, icon => icon.EndsWith("BTNPeasant.dds", StringComparison.OrdinalIgnoreCase));
        var models = files.List("", "sd", ".mdx");
        Assert.True(models.Count > 1000);
        Assert.DoesNotContain(models, model => model.Contains(".w3mod:", StringComparison.OrdinalIgnoreCase));
    }

    /// <summary>The install, opened once for the class.</summary>
    public sealed class Install : IDisposable
    {
        public Install()
        {
            Folder = GameInstall.Find();
            if (Folder == null)
            {
                Reason = $"No Warcraft III install in {string.Join(", ", GameInstall.Places())}; set {GameInstall.Variable} to run these tests";
                return;
            }

            Files = GameFiles.Open(Folder);
        }

        public string? Folder { get; }

        public GameFiles? Files { get; }

        public string? Reason { get; }

        public void Dispose()
        {
            Files?.Dispose();
        }
    }
}
