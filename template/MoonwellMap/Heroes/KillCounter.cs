using Moonwell;

namespace MoonwellMap.Heroes
{
    /// <summary>Counts the unit's kills, tells its owner each time, and heals the unit a tenth of its life.</summary>
    public sealed class KillCounter : UnitComponent
    {
        private int kills;

        public override void OnKill(Unit killedUnit)
        {
            kills++;
            Unit.Player.Print("Kills: " + kills);
            Unit.Life += Unit.MaxLife * 0.1f;
        }
    }
}
