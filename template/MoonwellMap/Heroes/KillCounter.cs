using Moonwell;

namespace MoonwellMap.Heroes
{
    /// <summary>Counts the unit's kills and tells its owner each time.</summary>
    public sealed class KillCounter : UnitComponent
    {
        private int kills;

        public override void OnKill(Unit victim)
        {
            kills++;
            Unit.Player.Print("Kills: " + kills);
        }
    }
}
