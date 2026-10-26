namespace Moonwell.Hosting
{
    /// <summary>The game as the terrain package sees it: the ground's height, the four pathing flags, the item trick that finds where a unit would land, and the playable area. Types are the game's numbers.</summary>
    public interface ITerrainWorld
    {
        /// <summary>The height of the ground at a point as the game reads it, deformations included. The game can answer differently on each machine, so a map reads it only inside <c>Local.For</c>.</summary>
        [MachineLocal]
        float LocationZ(float x, float y);

        /// <summary>Whether units can walk at a point.</summary>
        bool IsWalkable(float x, float y);

        /// <summary>Whether flying units can fly at a point.</summary>
        bool IsFlyable(float x, float y);

        /// <summary>Whether a building can be built at a point.</summary>
        bool IsBuildable(float x, float y);

        /// <summary>Whether ships can float at a point.</summary>
        bool IsFloatable(float x, float y);

        /// <summary>Puts an item at a point and reads back where the game left it: the nearest place an item can lie.</summary>
        void Place(float x, float y, out float landedX, out float landedY);

        /// <summary>The playable area: the map's bounds less the camera's margins.</summary>
        void PlayableArea(out float minX, out float minY, out float maxX, out float maxY);
    }
}
