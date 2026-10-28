using System.Buffers.Binary;
using System.Text;

namespace Moonwell.MapFiles;

/// <summary>
/// Four-character ids (rawcodes) as War3Net stores them: one int, the first character in the lowest byte.
/// </summary>
public static class Rawcodes
{
    /// <summary>The four characters an id integer holds; trailing NULs are dropped, so 0 is <c>""</c>.</summary>
    public static string TextOf(int id)
    {
        var bytes = new byte[4];
        BinaryPrimitives.WriteInt32LittleEndian(bytes, id);
        return Encoding.ASCII.GetString(bytes).TrimEnd('\0');
    }

    /// <summary>The integer of an id's four characters, such as <c>hfoo</c>; an empty text is 0.</summary>
    public static int IdOf(string text)
    {
        var bytes = new byte[4];
        Encoding.ASCII.GetBytes(text, 0, Math.Min(4, text.Length), bytes, 0);
        return BinaryPrimitives.ReadInt32LittleEndian(bytes);
    }

    /// <summary>
    /// The integer for a four-character id; throws when the text is not four characters of one byte each.
    /// </summary>
    /// <param name="text">The id's text.</param>
    /// <param name="name">The name of the parameter or field the text came from, for the exception.</param>
    /// <exception cref="ArgumentException">The text is null, not four characters long, or has a character over one byte.</exception>
    public static int From(string? text, string name)
    {
        if (text is null || text.Length != 4 || text.Any(c => c > 0xFF))
        {
            throw new ArgumentException($"An id must be four characters, not \"{text}\".", name);
        }

        return IdOf(text);
    }
}
