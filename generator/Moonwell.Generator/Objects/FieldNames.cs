using System.Text;

namespace Moonwell.Generator.Objects;

/// <summary>A field waiting for its C# property name: its id, the name its label gives, and the classes it is in.</summary>
/// <param name="Id">The field id, appended to the name when two fields of one class would share it.</param>
/// <param name="Name">The name the field asks for.</param>
/// <param name="Listed">True when the name is hand-picked, so it is kept whatever it meets.</param>
/// <param name="Groups">
/// The classes the field is a property of. Two fields can share a name only when they have no group in common.
/// </param>
public sealed record NameCandidate(string Id, string Name, bool Listed, IReadOnlyList<string> Groups);

/// <summary>The C# property names of fields: the hand-picked ones, and names made from World Editor's labels.</summary>
public static class FieldNames
{
    private static readonly string[] Reserved =
    {
        "Equals", "Finalize", "GetHashCode", "GetType", "MemberwiseClone", "ReferenceEquals", "ToString",
    };

    private static readonly Lazy<Dictionary<Kind, Dictionary<string, string>>> Listed = new(ReadListed);

    /// <summary>
    /// The PascalCase of the label's words: letters and digits only, apostrophes dropped, a leading digit prefixed
    /// with <c>_</c> (<c>Hit Points Maximum (Base)</c> is <c>HitPointsMaximumBase</c>). A label with no letter or
    /// digit gives an empty string.
    /// </summary>
    public static string FromLabel(string label)
    {
        var name = new StringBuilder();
        var startOfWord = true;
        foreach (var c in label)
        {
            if (c == '\'' || c == '’')
            {
                continue;
            }

            if (!char.IsAsciiLetterOrDigit(c))
            {
                startOfWord = true;
                continue;
            }

            name.Append(startOfWord ? char.ToUpperInvariant(c) : c);
            startOfWord = false;
        }

        if (name.Length > 0 && char.IsAsciiDigit(name[0]))
        {
            name.Insert(0, '_');
        }

        return name.ToString();
    }

    /// <summary>The hand-picked names of a kind's fields by field id, from the embedded <c>field-names.txt</c>.</summary>
    public static IReadOnlyDictionary<string, string> HandPicked(Kind kind)
    {
        return Listed.Value.TryGetValue(kind, out var names) ? names : new Dictionary<string, string>();
    }

    /// <summary>
    /// The final names of the candidates, in their order. A name two candidates share in a group gets the field id
    /// appended (<c>AreaOfEffect</c> and <c>AreaOfEffect</c> become <c>AreaOfEffectAare</c> and <c>AreaOfEffectHbz2</c>)
    /// except a hand-picked one, which keeps its name; a name that is a member of <c>object</c> gets the id too.
    /// </summary>
    /// <exception cref="InvalidOperationException">A name is still shared after that.</exception>
    public static IReadOnlyList<string> Assign(IReadOnlyList<NameCandidate> candidates)
    {
        var names = new string[candidates.Count];
        var first = new Dictionary<(string, string), int>();
        var shared = new HashSet<int>();
        for (var i = 0; i < candidates.Count; i++)
        {
            names[i] = candidates[i].Name;
            foreach (var group in candidates[i].Groups)
            {
                if (first.TryGetValue((group, candidates[i].Name), out var other))
                {
                    shared.Add(i);
                    shared.Add(other);
                }
                else
                {
                    first[(group, candidates[i].Name)] = i;
                }
            }

            if (Array.IndexOf(Reserved, candidates[i].Name) >= 0)
            {
                shared.Add(i);
            }
        }

        foreach (var i in shared)
        {
            if (!candidates[i].Listed)
            {
                names[i] = candidates[i].Name + Capitalize(candidates[i].Id);
            }
        }

        var taken = new Dictionary<(string, string), int>();
        for (var i = 0; i < candidates.Count; i++)
        {
            foreach (var group in candidates[i].Groups)
            {
                if (taken.TryGetValue((group, names[i]), out var other))
                {
                    throw new InvalidOperationException(
                        $"fields {candidates[other].Id} and {candidates[i].Id} are both named {names[i]}");
                }

                taken[(group, names[i])] = i;
            }
        }

        return names;
    }

    private static string Capitalize(string text)
    {
        return text.Length == 0 ? text : char.ToUpperInvariant(text[0]) + text.Substring(1);
    }

    private static Dictionary<Kind, Dictionary<string, string>> ReadListed()
    {
        var assembly = typeof(FieldNames).Assembly;
        using var stream = assembly.GetManifestResourceStream("field-names.txt")
            ?? throw new InvalidOperationException("field-names.txt is not embedded in " + assembly.GetName().Name);
        using var reader = new StreamReader(stream, Encoding.UTF8);
        var result = new Dictionary<Kind, Dictionary<string, string>>();
        string? line;
        var number = 0;
        while ((line = reader.ReadLine()) != null)
        {
            number++;
            if (line.Trim().Length == 0)
            {
                continue;
            }

            var parts = line.Split(' ', StringSplitOptions.RemoveEmptyEntries);
            if (parts.Length != 3 || !Enum.TryParse<Kind>(parts[0], true, out var kind))
            {
                throw new FormatException($"field-names.txt line {number}: '{line}' is not '<kind> <field id> <Name>'");
            }

            if (!result.TryGetValue(kind, out var names))
            {
                result[kind] = names = new Dictionary<string, string>(StringComparer.Ordinal);
            }

            if (!names.TryAdd(parts[1], parts[2]))
            {
                throw new FormatException($"field-names.txt line {number}: {parts[0]} {parts[1]} is listed twice");
            }
        }

        return result;
    }
}
