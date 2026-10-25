namespace Moonwell.Hosting
{
    /// <summary>The game as the interactions package sees it: whether a unit, item or destructable has a place, and the world's bounds. Types are the game's.</summary>
    public interface IInteractionWorld
    {
        /// <summary>Whether a unit is hidden.</summary>
        bool IsHidden(unit target);

        /// <summary>Whether a unit is loaded in a transport.</summary>
        bool IsLoaded(unit target);

        /// <summary>Whether an item is visible, which is whether it lies on the ground and not in a pack or a hidden place.</summary>
        bool IsVisible(item target);

        /// <summary>Whether a destructable is alive.</summary>
        bool IsAlive(destructable target);

        /// <summary>The world's bounds, read once when the grid is made.</summary>
        void WorldBounds(out float minX, out float minY, out float maxX, out float maxY);
    }
}
