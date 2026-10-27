namespace Moonwell.GameData;

/// <summary>The install's active build, from its <c>.build.info</c>.</summary>
/// <param name="Version">The game's version (<c>3.0.1.24371</c>).</param>
/// <param name="Key">The build key, which changes with every update of the game's files.</param>
/// <param name="Locale">The language of the installed text (<c>enUS</c>).</param>
public sealed record GameBuild(string Version, string Key, string Locale);

/// <summary>
/// Where a Warcraft III install is: a folder given, else the folder <see cref="Variable"/> names, else the usual folder.
/// A folder holds an install when it has <c>.build.info</c>.
/// </summary>
public static class GameInstall
{
    /// <summary>The environment variable that names the install's folder when none is given.</summary>
    public const string Variable = "MOONWELL_WAR3";

    private const string BuildInfo = ".build.info";

    /// <summary>The folder the game installs to by default.</summary>
    public static string UsualFolder =>
        Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFilesX86), "Warcraft III");

    /// <summary>
    /// Where the install is looked for: only <paramref name="folder"/> when one is given, else only the folder
    /// <see cref="Variable"/> names when it is set, else the usual folder.
    /// </summary>
    public static IReadOnlyList<string> Places(string? folder = null)
    {
        return Places(folder, Environment.GetEnvironmentVariable(Variable));
    }

    /// <summary>The install's folder (see <see cref="Places(string?)"/>); null when none of them holds one.</summary>
    public static string? Find(string? folder = null)
    {
        return Find(Places(folder));
    }

    /// <summary>The install's folder (see <see cref="Places(string?)"/>).</summary>
    /// <exception cref="DirectoryNotFoundException">No place holds an install; the message names each place.</exception>
    public static string Require(string? folder = null)
    {
        return Require(Places(folder));
    }

    /// <summary>The active build of the install in <paramref name="folder"/>.</summary>
    /// <exception cref="InvalidDataException"><c>.build.info</c> names no active build.</exception>
    public static GameBuild Build(string folder)
    {
        var lines = File.ReadAllLines(Path.Combine(folder, BuildInfo));
        var head = lines[0].Split('|').Select(column => column.Split('!')[0]).ToArray();
        foreach (var line in lines.Skip(1))
        {
            var row = head.Zip(line.Split('|')).ToDictionary(pair => pair.First, pair => pair.Second);
            if (row.GetValueOrDefault("Active") == "1")
            {
                return new GameBuild(
                    row.GetValueOrDefault("Version", ""),
                    row.GetValueOrDefault("Build Key", ""),
                    LocaleOf(row.GetValueOrDefault("Tags", "")));
            }
        }

        throw new InvalidDataException($"{Path.Combine(folder, BuildInfo)} names no active build");
    }

    internal static IReadOnlyList<string> Places(string? folder, string? variable)
    {
        if (!string.IsNullOrWhiteSpace(folder))
        {
            return new[] { folder };
        }

        return new[] { string.IsNullOrWhiteSpace(variable) ? UsualFolder : variable };
    }

    internal static string? Find(IEnumerable<string> places)
    {
        return places.FirstOrDefault(place => File.Exists(Path.Combine(place, BuildInfo)));
    }

    internal static string Require(IReadOnlyList<string> places)
    {
        return Find(places) ?? throw new DirectoryNotFoundException(
            $"No Warcraft III install (a folder with {BuildInfo}) in {string.Join(", ", places)}; "
            + $"set {Variable} to the game's folder.");
    }

    /// <summary>
    /// The locale of the installed text from the build's tags (<c>Windows EU? enUS speech?:Windows EU? enUS text?</c>);
    /// <c>enUS</c> when the tags name none.
    /// </summary>
    internal static string LocaleOf(string tags)
    {
        foreach (var tag in tags.Split('?', ':'))
        {
            var words = tag.Trim().Split(' ');
            if (words.Length == 2 && words[1] == "text" && words[0].Length == 4)
            {
                return words[0];
            }
        }

        return "enUS";
    }
}
