namespace Moonwell
{
    /// <summary>Finds the units the game has removed, which fire no event, and ends them. A timer runs it every 0.25 seconds.</summary>
    internal static class Sweep
    {
        /// <summary>Ends each live unit whose type id reads 0, which means the game has removed it.</summary>
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
        }
    }
}
