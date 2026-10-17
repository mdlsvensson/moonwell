using System.Text.RegularExpressions;

namespace Moonwell.Generator;

public sealed record JassType(string Name, string Parent);

public sealed record JassParameter(string Type, string Name);

public sealed record JassFunction(string Name, IReadOnlyList<JassParameter> Parameters, string Returns, string Source);

public sealed record JassGlobal(string Name, string Type, bool IsConstant, bool IsArray, string? Value, string Source);

public sealed class JassScripts
{
    public List<JassType> Types { get; } = new();
    public List<JassFunction> Functions { get; } = new();
    public List<JassGlobal> Globals { get; } = new();
}

public static partial class Jass
{
    public static JassScripts Read(string commonJ, string blizzardJ)
    {
        var scripts = new JassScripts();
        ReadInto(scripts, commonJ, "common.j");
        ReadInto(scripts, blizzardJ, "blizzard.j");
        return scripts;
    }

    public static void ReadInto(JassScripts scripts, string text, string source)
    {
        var inGlobals = false;
        foreach (var rawLine in text.Split('\n'))
        {
            var line = StripComment(rawLine).Trim();
            if (line.Length == 0)
            {
                continue;
            }

            if (line == "globals")
            {
                inGlobals = true;
                continue;
            }

            if (line == "endglobals")
            {
                inGlobals = false;
                continue;
            }

            if (inGlobals)
            {
                scripts.Globals.Add(ReadGlobal(line, source));
                continue;
            }

            var type = TypeLine().Match(line);
            if (type.Success)
            {
                scripts.Types.Add(new JassType(type.Groups["name"].Value, type.Groups["parent"].Value));
                continue;
            }

            var function = FunctionLine().Match(line);
            if (function.Success)
            {
                scripts.Functions.Add(new JassFunction(
                    function.Groups["name"].Value,
                    ReadParameters(function.Groups["takes"].Value),
                    function.Groups["returns"].Value,
                    source));
            }
        }
    }

    private static JassGlobal ReadGlobal(string line, string source)
    {
        var match = GlobalLine().Match(line);
        if (!match.Success)
        {
            throw new FormatException($"{source}: cannot read the global \"{line}\"");
        }

        var value = match.Groups["value"].Success ? match.Groups["value"].Value.Trim() : null;
        return new JassGlobal(
            match.Groups["name"].Value,
            match.Groups["type"].Value,
            match.Groups["constant"].Success,
            match.Groups["array"].Success,
            value,
            source);
    }

    private static IReadOnlyList<JassParameter> ReadParameters(string takes)
    {
        if (takes == "nothing")
        {
            return Array.Empty<JassParameter>();
        }

        return takes.Split(',')
            .Select(part => part.Trim().Split(' ', StringSplitOptions.RemoveEmptyEntries))
            .Select(words => new JassParameter(words[0], words[1]))
            .ToList();
    }

    private static string StripComment(string line)
    {
        var inString = false;
        for (var i = 0; i < line.Length - 1; i++)
        {
            if (line[i] == '"' && (i == 0 || line[i - 1] != '\\'))
            {
                inString = !inString;
            }

            if (!inString && line[i] == '/' && line[i + 1] == '/')
            {
                return line[..i];
            }
        }

        return line;
    }

    [GeneratedRegex(@"^type\s+(?<name>\w+)\s+extends\s+(?<parent>\w+)$")]
    private static partial Regex TypeLine();

    [GeneratedRegex(@"^(?:constant\s+)?(?:native|function)\s+(?<name>\w+)\s+takes\s+(?<takes>.+?)\s+returns\s+(?<returns>\w+)$")]
    private static partial Regex FunctionLine();

    [GeneratedRegex(@"^(?<constant>constant\s+)?(?<type>\w+)\s+(?<array>array\s+)?(?<name>\w+)(?:\s*=\s*(?<value>.+))?$")]
    private static partial Regex GlobalLine();
}
