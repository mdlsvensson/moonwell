namespace Moonwell
{
    /// <summary>Finds the units, items and destructables the game has removed, which fire no event, and ends them. A timer runs it every 0.25 seconds.</summary>
    internal static class Sweep
    {
        /// <summary>Ends each live unit, item and destructable whose type id reads 0, which means the game has removed it.</summary>
        internal static void Run()
        {
            var units = Units.Live.Snapshot();
            for (var i = 0; i < units.Count; i++)
            {
                var unit = units[i];
                if (Runtime.World.UnitTypeId(unit.Handle) == 0)
                {
                    unit.IsGone = true;
                    unit.End();
                }
            }

            var items = Items.Live.Snapshot();
            for (var i = 0; i < items.Count; i++)
            {
                var item = items[i];
                if (Runtime.World.ItemTypeId(item.Handle) == 0)
                {
                    item.IsGone = true;
                    item.End();
                }
            }

            var destructables = Destructables.Live.Snapshot();
            for (var i = 0; i < destructables.Count; i++)
            {
                var destructable = destructables[i];
                if (Runtime.World.DestructableTypeId(destructable.Handle) == 0)
                {
                    destructable.IsGone = true;
                    destructable.End();
                }
            }
        }
    }
}
