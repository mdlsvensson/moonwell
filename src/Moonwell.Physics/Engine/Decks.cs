using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>
    /// The decks of the map's placed bridges, which are ground where they stand. The build writes each placed bridge (its
    /// type, its deck's shape, its place, facing and scale, whether its type turns with its facing, and the ground beneath
    /// any water at the place the game makes it) and the shapes of the types placed, as text. At the start each bridge's
    /// handle is found near its place, and its deck is turned and scaled about the place the game reports, on the ground
    /// beneath any water there; a deck counts until its bridge dies or leaves the game. The ground asks the living decks
    /// whose box holds a point, in the order the build wrote them; it asks only when there is one.
    /// </summary>
    internal static class Decks
    {
        /// <summary>A shape's value where it has no deck.</summary>
        internal const float None = -1000000.0f;

        /// <summary>The decks, in the order the build wrote the bridges; null until the first is made. It holds no null, so the reads take it raw.</summary>
        private static List<Deck>? decks;

        /// <summary>How many decks are living; the ground asks them only when there is one.</summary>
        internal static int count;

        /// <summary>Forgets every deck; ResetSettings calls it. A deck forgotten counts no more, even if its bridge's watch reports it later.</summary>
        internal static void Reset()
        {
            var list = decks;
            if (list != null)
            {
                for (var i = 0; i < list.Count; i++)
                {
                    Raw.At(list, i).living = false;
                }
            }

            decks = null;
            count = 0;
        }

        /// <summary>
        /// Makes the decks of the bridges the build wrote. A shape is "cell columns rows originColumn originRow" and then its
        /// values row by row, each a height or "." for none. A bridge is "type shape x y facing scale turns ground": its
        /// four-character type, the number of its shape in the order written, its place in war3map.doo, its facing in degrees,
        /// its scale, 1 when its type turns with its facing (fixed rotation -1), else 0, and the ground's height beneath any
        /// water at the place the game makes it, which its deck stands on over water. A bridge whose handle is not found near
        /// its place has no deck.
        /// </summary>
        internal static void Load(IPhysicsWorld world, string bridges, string shapes)
        {
            var shapeValues = new List<float[]>();
            var shapeSizes = new List<float>();
            var reader = new Reader(shapes);
            while (reader.More())
            {
                var cell = reader.Number();
                var columns = (int)reader.Number();
                var rows = (int)reader.Number();
                shapeSizes.Add(cell);
                shapeSizes.Add(columns);
                shapeSizes.Add(rows);
                shapeSizes.Add(reader.Number());
                shapeSizes.Add(reader.Number());
                var values = new float[columns * rows];
                for (var i = 0; i < values.Length; i++)
                {
                    values[i] = reader.Value();
                }

                shapeValues.Add(values);
            }

            reader = new Reader(bridges);
            while (reader.More())
            {
                var type = reader.Id();
                var shape = (int)reader.Number();
                var x = reader.Number();
                var y = reader.Number();
                var facing = reader.Number();
                var scale = reader.Number();
                var turns = reader.Number() != 0.0f;
                var beneath = reader.Number();
                var handle = world.FindPlacedDestructable(type, x, y);
                var bridge = handle == null ? null : Destructable.Of(handle);
                if (bridge == null || bridge.HasEnded || bridge.ending)
                {
                    continue;
                }

                var sizes = shape * 5;
                var deck = new Deck(
                    bridge,
                    shapeValues[shape],
                    shapeSizes[sizes],
                    (int)shapeSizes[sizes + 1],
                    (int)shapeSizes[sizes + 2],
                    (int)shapeSizes[sizes + 3],
                    (int)shapeSizes[sizes + 4],
                    turns ? facing - 270.0f : 0.0f,
                    scale,
                    beneath);
                if (decks == null)
                {
                    decks = new List<Deck>();
                }

                Raw.Append(decks, deck);
                count++;
                bridge.Add(new DeckWatch(deck));
            }
        }

        /// <summary>A deck's bridge has died or left the game: the deck is ground no more, and what rests on it wakes, to fall.</summary>
        internal static void Gone(Deck deck)
        {
            if (!deck.living)
            {
                return;
            }

            deck.living = false;
            count--;
            Pass.GroundGone(deck.minX, deck.minY, deck.maxX, deck.maxY);
        }

        /// <summary>Whether a living deck's box holds a point: something there may meet a deck that the terrain's height does not show.</summary>
        internal static bool Near(float x, float y)
        {
            var list = decks;
            if (list == null)
            {
                return false;
            }

            var length = list.Count;
            for (var i = 0; i < length; i++)
            {
                var deck = Raw.At(list, i);
                if (deck.living && x >= deck.minX && x <= deck.maxX && y >= deck.minY && y <= deck.maxY)
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>Whether a destructable is a placed bridge with a deck, living or not: missiles do not meet it as a destructable.</summary>
        internal static bool IsBridge(Destructable destructable)
        {
            var list = decks;
            if (list == null)
            {
                return false;
            }

            var length = list.Count;
            for (var i = 0; i < length; i++)
            {
                if (object.ReferenceEquals(Raw.At(list, i).bridge, destructable))
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>Gives the height of the highest deck at a point that is not above a height, so a deck is met from above only; false when there is none.</summary>
        internal static bool Top(float x, float y, float z, out float height)
        {
            return Pick(x, y, z, true, out height) != null;
        }

        /// <summary>Gives the height of the highest deck at a point, for a ground unit, which stands on a deck where there is one; false when there is none.</summary>
        internal static bool Under(float x, float y, out float height)
        {
            return Pick(x, y, 0.0f, false, out height) != null;
        }

        /// <summary>Whether a living deck is at a point, whatever its height there: Under's test with no height read, for the points of a pushed unit's edge.</summary>
        internal static bool Covers(float x, float y)
        {
            var list = decks;
            if (list == null)
            {
                return false;
            }

            var length = list.Count;
            for (var i = 0; i < length; i++)
            {
                var deck = Raw.At(list, i);
                if (deck.living && x >= deck.minX && x <= deck.maxX && y >= deck.minY && y <= deck.maxY && deck.Covers(x, y))
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>Gives the normal of the deck a height was read from at a point, of length 1; false when the height there is the terrain's.</summary>
        internal static bool Normal(float x, float y, float z, out float nx, out float ny, out float nz)
        {
            nx = 0.0f;
            ny = 0.0f;
            nz = 1.0f;
            float height;
            var deck = Pick(x, y, z, true, out height);
            if (deck == null || !(height > Terrain.Height(x, y)))
            {
                return false;
            }

            deck.Normal(x, y, out nx, out ny, out nz);
            return true;
        }

        /// <summary>The highest living deck at a point, not above a height when below is set, and its height there; null when there is none.</summary>
        private static Deck? Pick(float x, float y, float z, bool below, out float height)
        {
            height = 0.0f;
            var list = decks;
            if (list == null)
            {
                return null;
            }

            Deck? best = null;
            var length = list.Count;
            for (var i = 0; i < length; i++)
            {
                var deck = Raw.At(list, i);
                if (!deck.living || x < deck.minX || x > deck.maxX || y < deck.minY || y > deck.maxY)
                {
                    continue;
                }

                float at;
                if (deck.Height(x, y, out at) && (!below || at <= z) && (best == null || at > height))
                {
                    best = deck;
                    height = at;
                }
            }

            return best;
        }

        /// <summary>Reads the text the build wrote, a number or a type at a time.</summary>
        private sealed class Reader
        {
            /// <summary>The text.</summary>
            private readonly string text;

            /// <summary>Where the reading is in the text.</summary>
            private int at;

            /// <summary>Starts reading a text from its beginning.</summary>
            internal Reader(string text)
            {
                this.text = text;
            }

            /// <summary>Whether the text has more to read once spaces and line ends are passed.</summary>
            internal bool More()
            {
                Skip();
                return at < text.Length;
            }

            /// <summary>Passes the spaces and line ends at the reading place.</summary>
            private void Skip()
            {
                var length = text.Length;
                while (at < length)
                {
                    var code = (int)text[at];
                    if (code != 32 && code != 10 && code != 13)
                    {
                        return;
                    }

                    at++;
                }
            }

            /// <summary>Reads a four-character type, as an integer with the first character highest.</summary>
            internal int Id()
            {
                Skip();
                var id = ((int)text[at] << 24) | ((int)text[at + 1] << 16) | ((int)text[at + 2] << 8) | (int)text[at + 3];
                at += 4;
                return id;
            }

            /// <summary>Reads a shape's value: a number, or None for ".".</summary>
            internal float Value()
            {
                Skip();
                if ((int)text[at] == 46)
                {
                    at++;
                    return None;
                }

                return Number();
            }

            /// <summary>Reads a number as the build writes it: an optional minus, digits, and an optional point and digits.</summary>
            internal float Number()
            {
                Skip();
                var length = text.Length;
                var negative = false;
                if (at < length && (int)text[at] == 45)
                {
                    negative = true;
                    at++;
                }

                var value = 0.0f;
                while (at < length && Digit((int)text[at]))
                {
                    value = value * 10.0f + ((int)text[at] - 48);
                    at++;
                }

                if (at < length && (int)text[at] == 46)
                {
                    at++;
                    var place = 0.1f;
                    while (at < length && Digit((int)text[at]))
                    {
                        value += ((int)text[at] - 48) * place;
                        place *= 0.1f;
                        at++;
                    }
                }

                return negative ? -value : value;
            }

            /// <summary>Whether a character code is a digit.</summary>
            private static bool Digit(int code)
            {
                return code >= 48 && code <= 57;
            }
        }
    }
}
