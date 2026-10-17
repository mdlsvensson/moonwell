namespace Moonwell.Testing
{
    /// <summary>A burn that stays on through its unit's death.</summary>
    public class DeathlessBurn : Burn
    {
        /// <summary>Makes a burn that a death does not end.</summary>
        public DeathlessBurn()
        {
            EndsOnDeath = false;
        }
    }
}
