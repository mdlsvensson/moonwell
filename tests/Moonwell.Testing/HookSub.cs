namespace Moonwell.Testing
{
    /// <summary>A subclass of HookBase that overrides OnDamaged and nothing else.</summary>
    public class HookSub : HookBase
    {
        /// <summary>Overrides the reaction to damage.</summary>
        public override void OnDamaged(int amount)
        {
        }
    }
}
