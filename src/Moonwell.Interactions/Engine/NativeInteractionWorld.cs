using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>The game as the interactions package sees it, through the game's natives; the only class of the package that calls them.</summary>
    internal sealed class NativeInteractionWorld : IInteractionWorld
    {
        /// <summary>Whether a unit is hidden.</summary>
        public bool IsHidden(unit target) => Natives.IsUnitHidden(target);

        /// <summary>Whether a unit is loaded in a transport.</summary>
        public bool IsLoaded(unit target) => Natives.IsUnitLoaded(target);

        /// <summary>Whether an item is visible.</summary>
        public bool IsVisible(item target) => Natives.IsItemVisible(target);

        /// <summary>Whether a destructable is alive.</summary>
        public bool IsAlive(destructable target) => Natives.GetDestructableLife(target) > 0.0f;

        /// <summary>The world's bounds; GetWorldBounds makes a rect, which is read and left.</summary>
        public void WorldBounds(out float minX, out float minY, out float maxX, out float maxY)
        {
            var bounds = Natives.GetWorldBounds();
            minX = Natives.GetRectMinX(bounds);
            minY = Natives.GetRectMinY(bounds);
            maxX = Natives.GetRectMaxX(bounds);
            maxY = Natives.GetRectMaxY(bounds);
        }
    }
}
