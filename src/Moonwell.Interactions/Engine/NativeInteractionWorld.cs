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

        /// <summary>Whether a destructable type is a tree, by the table the build wrote.</summary>
        public bool IsTreeType(int typeId) => TreeTypeOf(typeId);

        // MoonwellTrees = { [typeId] = true }, written by the build only for a map that references the package; a read of a
        // table makes nothing. False when there is no table or no entry (a template gives nil for a missing table otherwise).
        /// @CSharpLua.Template = "(MoonwellTrees ~= nil and MoonwellTrees[{0}] == true)"
        private static bool TreeTypeOf(int typeId)
        {
            return false;
        }

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
