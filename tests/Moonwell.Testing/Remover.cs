namespace Moonwell.Testing
{
    /// <summary>A unit component that removes its unit when the unit takes damage.</summary>
    public class Remover : UnitComponent
    {
        /// <summary>Removes the unit from the game.</summary>
        public override void OnDamaged(Damage damage)
        {
            Unit.Remove();
        }
    }
}
