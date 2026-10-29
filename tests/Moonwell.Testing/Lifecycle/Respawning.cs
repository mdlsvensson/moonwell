namespace Moonwell.Testing
{
    /// <summary>A creep whose own OnDeath starts a respawn timer, and a timer it names itself the owner of, as an effect that should end with the corpse.</summary>
    public sealed class Respawning : Unit
    {
        /// <summary>How many times the respawn timer has run.</summary>
        public int Respawned;

        /// <summary>How many times the timer the unit owns has run.</summary>
        public int Lingered;

        /// <summary>Starts a respawn timer for 120 seconds, and a timer every second that the unit owns.</summary>
        public override void OnDeath(Unit? killerUnit)
        {
            Game.Instance.After(120.0f, () => { Respawned++; });
            Every(1.0f, () => { Lingered++; });
        }
    }
}
