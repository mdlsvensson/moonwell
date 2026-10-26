namespace Moonwell.Engine
{
    /// <summary>
    /// The decks of the map's placed bridges, which are ground where they stand. The ground asks them at every read; there
    /// are none yet, so the ground is the terrain alone, until the build writes the bridges.
    /// </summary>
    internal static class Decks
    {
        /// <summary>How many decks there are; the ground asks them only when there is one.</summary>
        internal static int count;

        /// <summary>Gives the height of the highest deck at a point that is not above a height, so a deck is met from above only; false when there is none.</summary>
        internal static bool Top(float x, float y, float z, out float height)
        {
            height = 0.0f;
            return false;
        }

        /// <summary>Gives the height of the deck at a point, for a ground unit, which stands on a deck where there is one; false when there is none.</summary>
        internal static bool Under(float x, float y, out float height)
        {
            height = 0.0f;
            return false;
        }

        /// <summary>Gives the normal of the deck a height was read from at a point, of length 1; false when the height there is the terrain's.</summary>
        internal static bool Normal(float x, float y, float z, out float nx, out float ny, out float nz)
        {
            nx = 0.0f;
            ny = 0.0f;
            nz = 1.0f;
            return false;
        }
    }
}
