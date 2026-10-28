using Moonwell.GameData;

namespace Moonwell.Generator.Tests;

public class GameInstallTests
{
    private const string Header =
        "Branch!STRING:0|Active!DEC:1|Build Key!HEX:16|CDN Key!HEX:16|Tags!STRING:0|Version!STRING:0|Product!STRING:0";

    [Fact]
    public void the_build_is_the_active_line_of_build_info()
    {
        using var folder = new TempFolder();
        File.WriteAllText(
            Path.Combine(folder.Path, ".build.info"),
            Header + "\n"
            + "us|0|00000000000000000000000000000000|0|Windows US? enUS text?|2.0.0.1|w3\n"
            + "eu|1|dd5e630e7a6ad6466075eb1c50e283af|0|Windows EU? enUS speech?:Windows EU? enUS text?|3.0.1.24371|w3\n");

        var build = GameInstall.Build(folder.Path);

        Assert.Equal("3.0.1.24371", build.Version);
        Assert.Equal("dd5e630e7a6ad6466075eb1c50e283af", build.Key);
        Assert.Equal("enUS", build.Locale);
    }

    [Fact]
    public void build_info_with_no_active_build_is_an_error()
    {
        using var folder = new TempFolder();
        File.WriteAllText(Path.Combine(folder.Path, ".build.info"), Header + "\nus|0|00|0||1.0|w3\n");

        var error = Assert.Throws<InvalidDataException>(() => GameInstall.Build(folder.Path));

        Assert.Contains("no active build", error.Message);
    }

    [Theory]
    [InlineData("Windows EU? acct-SWE? geoip-SE? enUS speech?:Windows EU? acct-SWE? geoip-SE? enUS text?", "enUS")]
    [InlineData("Windows US? deDE speech?:Windows US? deDE text?", "deDE")]
    [InlineData("Windows EU? enUS speech?", "enUS")]
    [InlineData("", "enUS")]
    public void the_locale_is_the_installed_text_locale(string tags, string locale)
    {
        Assert.Equal(locale, GameInstall.LocaleOf(tags));
    }

    [Fact]
    public void a_given_folder_then_the_variable_then_the_usual_folder_is_where_the_install_is_looked_for()
    {
        Assert.Equal(new[] { @"C:\given" }, GameInstall.Places(@"C:\given", @"C:\variable"));
        Assert.Equal(new[] { @"C:\variable" }, GameInstall.Places(null, @"C:\variable"));
        Assert.Equal(new[] { GameInstall.UsualFolder }, GameInstall.Places(null, null));
        Assert.Equal(new[] { GameInstall.UsualFolder }, GameInstall.Places(" ", ""));
    }

    [Fact]
    public void an_install_is_a_folder_with_build_info()
    {
        using var empty = new TempFolder();
        using var install = new TempFolder();
        File.WriteAllText(Path.Combine(install.Path, ".build.info"), Header + "\n");

        Assert.Null(GameInstall.Find(new[] { empty.Path }));
        Assert.Equal(install.Path, GameInstall.Find(new[] { install.Path }));
    }

    [Fact]
    public void no_install_is_an_error_that_names_where_it_looked_and_the_variable()
    {
        using var empty = new TempFolder();

        var error = Assert.Throws<DirectoryNotFoundException>(() => GameInstall.Require(new[] { empty.Path }, true));

        Assert.Contains(empty.Path, error.Message);
        Assert.Contains(GameInstall.Variable, error.Message);
    }

    [Fact]
    public void no_install_in_a_given_folder_is_an_error_that_names_only_the_folder()
    {
        using var empty = new TempFolder();

        var error = Assert.Throws<DirectoryNotFoundException>(() => GameInstall.Require(empty.Path));

        Assert.Contains(empty.Path, error.Message);
        Assert.DoesNotContain(GameInstall.Variable, error.Message);
    }

    private sealed class TempFolder : IDisposable
    {
        public TempFolder()
        {
            Path = System.IO.Path.Combine(System.IO.Path.GetTempPath(), "moonwell-install-" + Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(Path);
        }

        public string Path { get; }

        public void Dispose()
        {
            Directory.Delete(Path, true);
        }
    }
}
