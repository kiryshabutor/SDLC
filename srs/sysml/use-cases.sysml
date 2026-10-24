// Рисунок А.2 — диаграмма вариантов использования (SRS, раздел 2.2)
package Actors {
    part def DealHunter { doc /* Система «Охотник за скидками в Steam» */ }
    part def Guest;
    part def RegisteredUser;
    part def Developer;
    part def Scheduler;
    part def SteamService;
    part def ItadService;
    part def MailServer;
}

package UseCases {
    private import Actors::*;

    // Гость
    use case def 'Зарегистрироваться и подтвердить email' {
        doc /* REQ-FUNC-001, REQ-FUNC-002 */
        subject : DealHunter;
        actor 'Гость' : Guest;
        actor 'Почтовый сервер' : MailServer;
    }
    use case def 'Войти' {
        doc /* REQ-FUNC-003 */
        subject : DealHunter;
        actor 'Гость' : Guest;
    }
    use case def 'Восстановить пароль' {
        doc /* REQ-FUNC-004 */
        subject : DealHunter;
        actor 'Гость' : Guest;
        actor 'Почтовый сервер' : MailServer;
    }

    // Пользователь
    use case def 'Найти игру и добавить в вишлист' {
        doc /* REQ-FUNC-008, REQ-FUNC-009, REQ-FUNC-011 */
        subject : DealHunter;
        actor 'Пользователь' : RegisteredUser;
        actor 'Steam' : SteamService;
    }
    use case def 'Импортировать вишлист Steam' {
        doc /* REQ-FUNC-010 */
        subject : DealHunter;
        actor 'Пользователь' : RegisteredUser;
        actor 'Steam' : SteamService;
    }
    use case def 'Управлять вишлистом и условиями уведомлений' {
        doc /* REQ-FUNC-012…REQ-FUNC-016 */
        subject : DealHunter;
        actor 'Пользователь' : RegisteredUser;
    }
    use case def 'Проверить цены сейчас' {
        doc /* REQ-FUNC-020 */
        subject : DealHunter;
        actor 'Пользователь' : RegisteredUser;
        actor 'Steam' : SteamService;
    }
    use case def 'Просмотреть историю цен игры' {
        doc /* REQ-FUNC-021…REQ-FUNC-024 */
        subject : DealHunter;
        actor 'Пользователь' : RegisteredUser;
    }
    use case def 'Просмотреть уведомления' {
        doc /* REQ-FUNC-028 */
        subject : DealHunter;
        actor 'Пользователь' : RegisteredUser;
    }
    use case def 'Изменить настройки или удалить учётную запись' {
        doc /* REQ-FUNC-005…REQ-FUNC-007, REQ-FUNC-029 */
        subject : DealHunter;
        actor 'Пользователь' : RegisteredUser;
    }

    // Планировщик (время)
    use case def 'Собрать цены по расписанию' {
        doc /* REQ-FUNC-017…REQ-FUNC-019 */
        subject : DealHunter;
        actor 'Планировщик' : Scheduler;
        actor 'Steam' : SteamService;
        actor 'IsThereAnyDeal' : ItadService;
    }
    use case def 'Уведомить о скидках' {
        doc /* REQ-FUNC-025…REQ-FUNC-027 */
        subject : DealHunter;
        actor 'Планировщик' : Scheduler;
        actor 'Почтовый сервер' : MailServer;
    }

    // Разработчик
    use case def 'Сымитировать изменение цены' {
        doc /* REQ-FUNC-030 (тестовый режим) */
        subject : DealHunter;
        actor 'Разработчик' : Developer;
    }
}
