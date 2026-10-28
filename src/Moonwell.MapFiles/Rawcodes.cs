using System.Buffers.Binary;
using System.Text;

namespace Moonwell.MapFiles;

/// <summary>
/// Four-character ids (rawcodes) as War3Net stores them: one int, the first character in the lowest byte. Each
/// character is one byte (Latin-1: the character with that code), so every byte value 0 to 255 keeps its place.
/// </summary>
public static class Rawcodes
{
    /// <summary>
    /// The characters an id integer holds, one per byte with the lowest byte first; trailing NULs are dropped, so 0 is
    /// <c>""</c> and an id of <c>ab</c> followed by two NUL bytes is <c>ab</c>.
    /// </summary>
    public static string TextOf(int id)
    {
        var bytes = new byte[4];
        BinaryPrimitives.WriteInt32LittleEndian(bytes, id);
        return Encoding.Latin1.GetString(bytes).TrimEnd('\0');
    }

    /// <summary>
    /// The integer of an id's characters, such as <c>hfoo</c>, one byte each with the first in the lowest byte. An empty
    /// text is 0, a shorter text is padded with 0 bytes and a longer one gives its first four characters. A character
    /// over one byte becomes <c>?</c>; <see cref="From"/> refuses such a text instead.
    /// </summary>
    public static int IdOf(string text)
    {
        var bytes = new byte[4];
        Encoding.Latin1.GetBytes(text, 0, Math.Min(4, text.Length), bytes, 0);
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

        return text[0] | text[1] << 8 | text[2] << 16 | text[3] << 24;
    }
}
