using System;
using System.Text;

namespace Moonwell.Testing
{
    /// <summary>Writes the terrain grid's text the way the build does, so a case can make a grid from corners: four characters a corner, row by row from the bottom left, each corner <c>h &lt;&lt; 7 | level &lt;&lt; 3 | ramp &lt;&lt; 1 | water</c> with <c>h</c> the height times 20 plus 49152.</summary>
    public static class GridText
    {
        /// <summary>The 64 characters, for the values 0 to 63.</summary>
        private const string Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

        /// <summary>The 17-bit number that stands for a height of 0.</summary>
        private const int ZeroHeight = 49152;

        /// <summary>The four characters of one corner.</summary>
        public static string Corner(float z, int level, bool ramp, bool water)
        {
            var h = (int)MathF.Floor(z * 20f + 0.5f) + ZeroHeight;
            var value = (h << 7) | (level << 3) | (ramp ? 2 : 0) | (water ? 1 : 0);
            var text = new StringBuilder();
            text.Append(Alphabet[(value >> 18) & 63]);
            text.Append(Alphabet[(value >> 12) & 63]);
            text.Append(Alphabet[(value >> 6) & 63]);
            text.Append(Alphabet[value & 63]);
            return text.ToString();
        }

        /// <summary>The text of a corner on dry ground, with no ramp.</summary>
        public static string Corner(float z, int level)
        {
            return Corner(z, level, false, false);
        }

        /// <summary>The text of a 2 by 2 grid, one tile: the corners bottom left, bottom right, top left, top right.</summary>
        public static string Tile(string bottomLeft, string bottomRight, string topLeft, string topRight)
        {
            return bottomLeft + bottomRight + topLeft + topRight;
        }
    }
}
