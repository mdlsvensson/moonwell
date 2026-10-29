using Moonwell.GameData;

namespace Moonwell.MapFiles;

/// <summary>
/// The rules for object ids, shared by Moonwell's build and Moonwell Editor: the ids World Editor gives new objects (a
/// letter for the kind, then the lowest free three characters, counting <c>0-9</c> then <c>A-Z</c> from <c>000</c>:
/// <c>h000</c>, <c>h001</c> ... <c>h009</c>, <c>h00A</c> ...), which ids can be typed, and which unit ids make a hero.
/// </summary>
public static class ObjectIds
{
    private const string Digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ";

    /// <summary>
    /// The first letter of a new object's id: <c>h</c> for a unit, <c>H</c> for one based on a hero (whose id starts
    /// with an upper-case letter), <c>I</c> for an item, <c>B</c> for a destructable, <c>D</c> for a doodad, <c>A</c>
    /// for an ability, <c>B</c> for a buff and <c>R</c> for an upgrade.
    /// </summary>
    /// <param name="kind">The kind of the new object.</param>
    /// <param name="baseId">The id of the object it is based on; only a unit's first character matters; a unit whose base id is empty gets <c>h</c>.</param>
    /// <returns>The letter.</returns>
    /// <exception cref="ArgumentOutOfRangeException"><paramref name="kind"/> is not one of the seven kinds.</exception>
    public static char Letter(Kind kind, string baseId) => kind switch
    {
        Kind.Unit => IsHeroId(baseId) ? 'H' : 'h',
        Kind.Item => 'I',
        Kind.Destructable => 'B',
        Kind.Doodad => 'D',
        Kind.Ability => 'A',
        Kind.Buff => 'B',
        Kind.Upgrade => 'R',
        _ => throw new ArgumentOutOfRangeException(nameof(kind), kind, null),
    };

    /// <summary>The lowest id with this letter that <paramref name="taken"/> does not accept; null when all are taken.</summary>
    /// <param name="letter">The first character of the id.</param>
    /// <param name="taken">Whether an id is already used; callers can combine several sets in it.</param>
    /// <returns>The id, counting from <c>000</c>, or null.</returns>
    public static string? Next(char letter, Func<string, bool> taken)
    {
        var count = Digits.Length;
        for (var number = 0; number < count * count * count; number++)
        {
            var id = string.Concat(
                letter,
                Digits[number / (count * count)],
                Digits[number / count % count],
                Digits[number % count]);
            if (!taken(id))
            {
                return id;
            }
        }

        return null;
    }

    /// <summary>Whether an id can be an object's id: four characters, each from <c>!</c> to <c>~</c> (an ASCII sign, letter or digit).</summary>
    /// <param name="id">The id.</param>
    /// <returns>True when it is valid.</returns>
    public static bool IsValid(string id) => id.Length == 4 && id.All(character => character >= '!' && character <= '~');

    /// <summary>
    /// Whether a unit with this id is a hero: the first character is an ASCII upper-case letter. The game makes such a
    /// unit a hero, and only such a unit.
    /// </summary>
    /// <param name="id">The unit's id.</param>
    /// <returns>True when the id starts with an upper-case letter.</returns>
    public static bool IsHeroId(string id) => id.Length > 0 && char.IsAsciiLetterUpper(id[0]);
}
