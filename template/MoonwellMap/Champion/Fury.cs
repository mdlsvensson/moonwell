using Moonwell;

namespace MoonwellMap
{
    /// <summary>Counts the unit's kills and tells its owner each time.</summary>
    public sealed class Fury : UnitComponent
    {
        private int kills;

        public override void OnKill(Unit victim)
        {
            kills++;
            Natives.DisplayTextToPlayer(Unit.Player.Handle, 0f, 0f, "Kills: " + kills);
        }
    }
}
